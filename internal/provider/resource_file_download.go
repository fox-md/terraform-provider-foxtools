// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"time"

	foxvalidators "github.com/fox-md/terraform-provider-validators"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
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
				Validators: []validator.String{
					foxvalidators.RestEndpointValidator{},
					HttpCacheValidator{},
				},
			},
			"filename": schema.StringAttribute{
				Description: "Local file download path.",
				Required:    true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(5),
				},
			},
			"file_chmod": schema.StringAttribute{
				Description: "Set file permissions. Works only on non-Windows OS.",
				Optional:    true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^[0-7]{3}$`), "Change mode is not valid."),
					ChmodValidator{},
				},
			},
			"headers": schema.MapAttribute{
				Description: "HTTP request headers.",
				Optional:    true,
				ElementType: types.StringType,
				Sensitive:   true,
			},
			"delete_on_destroy": schema.BoolAttribute{
				Description: "Delete file on destroy. Defaults to `true`.",
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
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
				Description: "Toogles to trigger file update (no need to configure manually).",
			},
			"etag": schema.StringAttribute{
				Description: "ETag header value.",
				Computed:    true,
			},
			"last_modified": schema.StringAttribute{
				Description: "Last-Modified header value.",
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
		resp.Diagnostics.AddError("Failed to download file", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *fileDownloadResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state fileResourceModel
	var isChmodUpToDate bool
	var localChmod string
	var headers map[string]string

	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	exists, err := fileExists(state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to check if file exists", err.Error())
		return
	}

	if !exists {
		resp.State.RemoveResource(ctx)
		return
	}

	diags = state.Headers.ElementsAs(ctx, &headers, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileAttributes, err := getLocalAndRemoteFilesAttrs(ctx, headers, state.URL.ValueString(), state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to check hashes", err.Error())
		return
	}

	if IsOSChmodCompat() {
		localChmod, err = getFileChmod(state.Filename.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error checking file permissions", err.Error())
			return
		}
	} else {
		localChmod = state.FileChmod.ValueString()
	}

	isFileUpToDate := fileAttributes.ETag == state.Etag.ValueString() &&
		fileAttributes.Sha256 == state.Sha256.ValueString() &&
		fileAttributes.LastModified == state.LastModified.ValueString()

	if !state.FileChmod.IsNull() && !state.FileChmod.IsUnknown() {
		isChmodUpToDate = localChmod == state.FileChmod.ValueString()
	} else {
		isChmodUpToDate = true
	}

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
	var planChmod string
	var stateChmod string

	var plan fileResourceModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state fileResourceModel
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

	fileAttributes, err := getLocalAndRemoteFilesAttrs(ctx, headers, plan.URL.ValueString(), state.Filename.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to check hashes", err.Error())
		return
	}

	isFileUpToDate := fileAttributes.ETag == state.Etag.ValueString() &&
		fileAttributes.Sha256 == state.Sha256.ValueString() &&
		fileAttributes.LastModified == state.LastModified.ValueString()

	if state.Filename.ValueString() != plan.Filename.ValueString() && isFileUpToDate {
		err := os.Rename(state.Filename.ValueString(), plan.Filename.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error moving file:", err.Error())
			return
		}
	}

	if !state.FileChmod.IsNull() && !state.FileChmod.IsUnknown() {
		stateChmod = state.FileChmod.ValueString()
	} else {
		stateChmod = ""
	}

	if !plan.FileChmod.IsNull() && !plan.FileChmod.IsUnknown() {
		planChmod = plan.FileChmod.ValueString()
	} else {
		planChmod = ""
	}

	if (!triggers["chmod_up_to_date"] || stateChmod != planChmod) && isFileUpToDate && planChmod != "" {
		err := setFileChmod(plan.Filename.ValueString(), plan.FileChmod.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to change file permissions:", err.Error())
			return
		}
	}

	if !triggers["download_file"] || !isFileUpToDate {
		err := downloadFile(ctx, &plan)
		if err != nil {
			resp.Diagnostics.AddError("Failed to download file", err.Error())
			return
		}
	} else {
		plan.Etag = types.StringValue(fileAttributes.ETag)
		plan.Sha256 = types.StringValue(fileAttributes.Sha256)
		plan.LastModified = types.StringValue(fileAttributes.LastModified)
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

	if state.DeleteOnDestroy.ValueBool() {
		err := os.Remove(state.Filename.ValueString())
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			resp.Diagnostics.AddError(
				"Error deleting file",
				"Could not delete file, unexpected error: "+err.Error(),
			)
			return
		}
	}
}

type fileResourceModel struct {
	URL               types.String `tfsdk:"url"`
	Filename          types.String `tfsdk:"filename"`
	FileChmod         types.String `tfsdk:"file_chmod"`
	Headers           types.Map    `tfsdk:"headers"`
	Etag              types.String `tfsdk:"etag"`
	LastModified      types.String `tfsdk:"last_modified"`
	Sha256            types.String `tfsdk:"sha256"`
	DownloadTrigger   types.Map    `tfsdk:"download_trigger"`
	DownloadTimestamp types.String `tfsdk:"download_timestamp"`
	DeleteOnDestroy   types.Bool   `tfsdk:"delete_on_destroy"`
}

func downloadFile(ctx context.Context, model *fileResourceModel) error {
	headers := make(map[string]string)
	for k, v := range model.Headers.Elements() {
		if strVal, ok := v.(types.String); ok {
			headers[k] = strVal.ValueString()
		}
	}

	fileAttributes, err := httpFileDownload(ctx, model.URL.ValueString(), model.Filename.ValueString(), headers)
	if err != nil {
		return fmt.Errorf("%s", err.Error())
	}

	if fileAttributes.ETag == "" && fileAttributes.LastModified == "" {
		return fmt.Errorf("both ETag and LastModified headers are empty. Got Etag = '%s', LastModified = '%s'", fileAttributes.ETag, fileAttributes.LastModified)
	}

	if !model.FileChmod.IsNull() && !model.FileChmod.IsUnknown() {
		err = setFileChmod(model.Filename.ValueString(), model.FileChmod.ValueString())
		if err != nil {
			return fmt.Errorf("%s", err.Error())
		}
	}

	model.Etag = types.StringValue(fileAttributes.ETag)
	model.Sha256 = types.StringValue(fileAttributes.Sha256)
	model.LastModified = types.StringValue(fileAttributes.LastModified)
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

func getLocalAndRemoteFilesAttrs(ctx context.Context, headers map[string]string, url string, filePath string) (fileAttributes, error) {
	response, err := getCachingHeaders(ctx, url, headers)
	if err != nil {
		return response, fmt.Errorf("failed to get Etag: %s", err.Error())
	}

	localSha256, err := SHA256File(filePath)
	if err != nil {
		return response, fmt.Errorf("failed to read local file hash: %s", err.Error())
	}

	response.Sha256 = localSha256
	return response, nil
}
