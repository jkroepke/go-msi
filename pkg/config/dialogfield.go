package config

// dialogField checks the type of a ui.dialogs field and the value of a
// checkbox.
func (v *validator) dialogField(cfg *Config, fieldPath string, field *DialogField) {
	v.oneOf(fieldPath+".type", field.Type, "", FieldText, FieldPassword, FieldCheckBox)

	if field.Value != "" && field.Type != FieldCheckBox {
		v.errorf(fieldPath+".value", "only a checkbox field has a value")
	}

	// A check box is drawn checked when its property has any value, so a
	// declared default such as "0" starts it checked (reported, not tested).
	if prop := cfg.Properties[field.Property]; field.Type == FieldCheckBox && prop != nil && prop.Value != "" {
		v.warnf(fieldPath+".property", "%s has the default %q, so the checkbox starts checked and leaves that value "+
			"when the user does not touch it; leave the default empty for an unchecked box", field.Property, prop.Value)
	}
}
