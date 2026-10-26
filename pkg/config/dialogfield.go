package config

// dialogField checks the type of a ui.dialogs field and the value of a
// checkbox.
func (v *validator) dialogField(fieldPath string, field *DialogField) {
	v.oneOf(fieldPath+".type", field.Type, "", FieldText, FieldPassword, FieldCheckBox)

	if field.Value != "" && field.Type != FieldCheckBox {
		v.errorf(fieldPath+".value", "only a checkbox field has a value")
	}
}
