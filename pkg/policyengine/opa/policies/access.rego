package auth

is_user_allowed_check(user, model) if {

	every attribute_name, allowed_values in model.accessPolicy.userAttributes {
		value := user.attributes[attribute_name]
		some allowed_value in allowed_values
		allowed_value == value
	}

} else := false

is_user_allowed := true if {
	is_user_allowed_check(input.user_obj, input.model_obj) == true
} else := false


allowed_models contains model_id if {
	some model in input.model_objs
	model_id := model.id
	is_user_allowed_check(input.user_obj, model)
}
