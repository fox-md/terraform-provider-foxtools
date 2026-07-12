// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

type fileDownloadResource struct{}

func NewFileDownloadResource() resource.Resource {
	return &fileDownloadResource{}
}

func (r *fileDownloadResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file_download"
}

func (r *fileDownloadResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Download a file via HTTP using GET",
		Attributes: map[string]schema.Attribute{
			"url": schema.StringAttribute{
				Description: "URL path to download file from.",
				Required:    true,
			},
			"filename": schema.StringAttribute{
				Description: "Local file download path.",
				Required:    true,
			},
			"file_chmod": schema.StringAttribute{
				Description: "File permissions. Defaults to `644`.",
				Default:     stringdefault.StaticString("644"),
				Optional:    true,
				Computed:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[0-7]{3}$`), "Change mode is not valid."),
				},
			},
			"headers": schema.MapAttribute{
				Description: "Request headers.",
				Optional:    true,
				ElementType: types.StringType,
				Sensitive:   true,
			},
			"download_trigger": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.BoolType,
				Default: mapdefault.StaticValue(
					types.MapValueMust(
						types.BoolType,
						map[string]attr.Value{
							"download_file":    types.BoolValue(true),
							"chmod_up_to_date": types.BoolValue(true),
						},
					),
				),
				Description: "Toogles to trigger file re-download (no need to configure manually).",
			},
			"etag": schema.StringAttribute{
				Description: "ETag value of the remote file.",
				Computed:    true,
			},
			"sha256": schema.StringAttribute{
				Description: "SHA256 sum of the local file.",
				Computed:    true,
			},
			"download_timestamp": schema.StringAttribute{
				Description: "Timestamp of file download.",
				Computed:    true,
			},
		},
	}
}

func (r *fileDownloadResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan fileResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := downloadFile(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("failed to download file", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *fileDownloadResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state fileResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	outputPath := state.Filename.ValueString()
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		resp.State.RemoveResource(ctx)
		return
	}

	var headers map[string]string
	diags = state.Headers.ElementsAs(ctx, &headers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileChecksums, err := getHashes(ctx, headers, state.URL.ValueString(), state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to check hashes", err.Error())
		return
	}

	localChmod, err := getFileChmod(state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error checking file permissions", err.Error())
		return
	}

	isFileUpToDate := fileChecksums.Etag == state.Etag.ValueString() && fileChecksums.sha256 == state.Sha256.ValueString()

	isChmodUpToDate := localChmod == state.FileChmod.ValueString()

	state.DownloadTrigger = types.MapValueMust(
		types.BoolType,
		map[string]attr.Value{
			"download_file":    types.BoolValue(isFileUpToDate),
			"chmod_up_to_date": types.BoolValue(isChmodUpToDate),
		},
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *fileDownloadResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan fileResourceModel
	var state fileResourceModel

	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	diags = req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var triggers map[string]bool
	diags = state.DownloadTrigger.ElementsAs(ctx, &triggers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var headers map[string]string
	diags = plan.Headers.ElementsAs(ctx, &headers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileChecksums, err := getHashes(ctx, headers, plan.URL.ValueString(), state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to check hashes", err.Error())
		return
	}

	isFileUpToDate := fileChecksums.Etag == state.Etag.ValueString() && fileChecksums.sha256 == state.Sha256.ValueString()

	if state.Filename.ValueString() != plan.Filename.ValueString() && isFileUpToDate {
		err := os.Rename(state.Filename.ValueString(), plan.Filename.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error moving file:", err.Error())
			return
		}
	}

	if (!triggers["chmod_up_to_date"] || state.FileChmod.ValueString() != plan.FileChmod.ValueString()) && isFileUpToDate {
		err := setFileChmod(plan.Filename.ValueString(), plan.FileChmod.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to change file permissions:", err.Error())
			return
		}
	}

	if !triggers["download_file"] || !isFileUpToDate {
		err := downloadFile(ctx, &plan)
		if err != nil {
			resp.Diagnostics.AddError("failed to download file", err.Error())
			return
		}
	} else {
		plan.Etag = types.StringValue(fileChecksums.Etag)
		plan.Sha256 = types.StringValue(fileChecksums.sha256)
		plan.DownloadTimestamp = types.StringValue(state.DownloadTimestamp.ValueString())
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *fileDownloadResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state fileResourceModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	err := os.Remove(state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting file",
			"Could not delete file, unexpected error: "+err.Error(),
		)
		return
	}
}

type fileResourceModel struct {
	URL               types.String `tfsdk:"url"`
	Filename          types.String `tfsdk:"filename"`
	FileChmod         types.String `tfsdk:"file_chmod"`
	Headers           types.Map    `tfsdk:"headers"`
	Etag              types.String `tfsdk:"etag"`
	Sha256            types.String `tfsdk:"sha256"`
	DownloadTrigger   types.Map    `tfsdk:"download_trigger"`
	DownloadTimestamp types.String `tfsdk:"download_timestamp"`
}

func downloadFile(ctx context.Context, model *fileResourceModel) error {
	headers := make(map[string]string)
	for k, v := range model.Headers.Elements() {
		if strVal, ok := v.(types.String); ok {
			headers[k] = strVal.ValueString()
		}
	}

	fileCheckSums, err := httpFileDownload(ctx, model.URL.ValueString(), model.Filename.ValueString(), headers)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	err = setFileChmod(model.Filename.ValueString(), model.FileChmod.ValueString())
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	model.Etag = types.StringValue(fileCheckSums.Etag)
	model.Sha256 = types.StringValue(fileCheckSums.sha256)
	model.DownloadTimestamp = types.StringValue(time.Now().Format(time.RFC3339Nano))
	model.DownloadTrigger = types.MapValueMust(
		types.BoolType,
		map[string]attr.Value{
			"download_file":    types.BoolValue(true),
			"chmod_up_to_date": types.BoolValue(true),
		},
	)
	return nil
}

func getHashes(ctx context.Context, headers map[string]string, url string, filePath string) (fileChecksums, error) {
	var fileChecksums fileChecksums

	localSha256, err := SHA256File(filePath)
	if err != nil {
		return fileChecksums, fmt.Errorf("failed to read local file hash: %s", err.Error())
	}

	etag, err := getEtag(ctx, url, headers)
	if err != nil {
		return fileChecksums, fmt.Errorf("failed to get Etag: %s", err.Error())
	}
	fileChecksums.sha256 = localSha256
	fileChecksums.Etag = etag
	return fileChecksums, nil
}
