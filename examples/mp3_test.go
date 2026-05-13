package examples

import (
	"testing"
	"yatori-go-console/utils"
)

func TestNotice(t *testing.T) {
	t.Skip("manual audio example; excluded from automated test runs")

	utils.PlayNoticeSound()
}
