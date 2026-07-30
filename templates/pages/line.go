package pages

import (
	"fmt"
	"net/url"
	"strings"
)

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

func dayLabel(offset int) string {
	switch offset {
	case 0:
		return "Hari ini"
	case 1:
		return "Besok"
	case 2:
		return "Lusa"
	default:
		return fmt.Sprintf("%d hari lagi", offset)
	}
}

func trainDetailURL(trainID, from, to int64, searchTime string) string {
	value := fmt.Sprintf("/train/%d?from=%d&to=%d", trainID, from, to)
	if searchTime != "" {
		value += "&time=" + url.QueryEscape(searchTime)
	}
	return value
}
