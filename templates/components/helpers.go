package components

func activeTimeClass(active bool) string {
	if active {
		return " is-active"
	}
	return ""
}
