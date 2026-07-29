package pages

import "strings"

func lineBadgeClass(route string) string {
	value := strings.ToUpper(route)
	switch {
	case strings.Contains(value, "TANGERANG"):
		return "line-tangerang"
	case strings.Contains(value, "TANJUNG PRIOK") || strings.Contains(value, "TANJUNGPRIUK"):
		return "line-priok"
	case strings.Contains(value, "RANGKAS"), strings.Contains(value, "MERAK"):
		return "line-rangkas"
	case strings.Contains(value, "YOGYAKARTA"), strings.Contains(value, "KUTOARJO"), strings.Contains(value, "PALUR"):
		return "line-regional"
	case strings.Contains(value, "BOGOR"), strings.Contains(value, "NAMBO"):
		return "line-bogor"
	default:
		return "line-cikarang"
	}
}
