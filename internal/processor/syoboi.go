package processor

import (
	"regexp"
	"strconv"
)

var (
	seasonRegex       = regexp.MustCompile(`\((\d+)\)$`)
	seasonSuffixRegex = regexp.MustCompile(`[sS]eason\s*(\d+)$`)
)

func cutSeasonFromSyoboiTitle(title string) (int, string) {
	// タイトルの末尾に "(X)" の形式でシーズン表記があるか確認する
	if seasonRegex.MatchString(title) {
		seasonNum, err := strconv.Atoi(seasonRegex.FindStringSubmatch(title)[1])
		removed := seasonRegex.ReplaceAllString(title, "")
		if err != nil {
			return 0, title
		}
		return seasonNum, removed
	}

	// タイトルの末尾に "Season X" があるか確認する
	if seasonSuffixRegex.MatchString(title) {
		seasonNum, err := strconv.Atoi(seasonSuffixRegex.FindStringSubmatch(title)[1])
		removed := seasonSuffixRegex.ReplaceAllString(title, "")
		if err != nil {
			return 0, title
		}
		return seasonNum, removed
	}
	return 0, title
}
