package tracemap

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fatih/color"
)

func GetMapUrl(r string) {
	url := "https://api.leo.moe/tracemap/api"
	resp, _ := http.Post(url, "application/json", strings.NewReader(r))
	body, _ := io.ReadAll(resp.Body)
	_, err := fmt.Fprintf(color.Output, "%s %s\n",
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "MapTrace URL:"),
		color.New(color.FgBlue, color.Bold).Sprintf("%s", string(body)),
	)
	if err != nil {
		return
	}
}
