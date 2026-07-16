// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ validator.String = HttpCacheValidator{}
)

type HttpCacheValidator struct {
	basetypes.StringValue
}

func (v HttpCacheValidator) Description(_ context.Context) string {
	return "ETag or Last-Modified headers validator."
}

func (v HttpCacheValidator) MarkdownDescription(_ context.Context) string {
	return "ETag or Last-Modified headers validator."
}

func (v HttpCacheValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	var headers map[string]string
	req.Config.GetAttribute(ctx, path.Root("headers"), &headers)
	url := req.ConfigValue.ValueString()

	valid, err := v.isValid(ctx, url, headers)

	if !valid {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Failed to check caching headers",
			err.Error(),
		)
	}
}

func (v HttpCacheValidator) isValid(ctx context.Context, url string, headers map[string]string) (bool, error) {
	fileAttributes, err := getCachingHeaders(ctx, url, headers)
	if err != nil {
		return false, err
	}
	if fileAttributes.ETag == "" && fileAttributes.LastModified == "" {
		return false, errors.New("Expected ETag or Last-Modified to have valid value. Both headers returned empty values.")
	}

	return true, nil
}
