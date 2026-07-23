// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var (
	_ validator.String = ChmodValidator{}
)

type ChmodValidator struct {
	basetypes.StringValue
}

func (v ChmodValidator) Description(_ context.Context) string {
	return "OS family chmod compatibility validator."
}

func (v ChmodValidator) MarkdownDescription(_ context.Context) string {
	return "OS family chmod compatibility validator."
}

func (v ChmodValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	if !IsOSChmodCompat() {
		resp.Diagnostics.AddAttributeWarning(
			req.Path,
			"Chmod incompatible OS",
			"Configured chmod is going to be ingnored.",
		)
	}
}
