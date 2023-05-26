package tracemap

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"io"
	"net/http"
	"strings"
)

func GetMapUrl(r string) (string, error) {
	url := "https://api.leo.moe/tracemap/api"
	resp, err := http.Post(url, "application/json", strings.NewReader(r))
	if err != nil {
		return "", errors.New("an issue occurred while connecting to the tracemap API")
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.New("an issue occurred while connecting to the tracemap API")
	}
	return string(body), nil
}

func PrintMapUrl(r string) {
	_, err := fmt.Fprintf(color.Output, "%s %s\n",
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "MapTrace URL:"),
		color.New(color.FgBlue, color.Bold).Sprintf("%s", string(r)),
	)
	if err != nil {
		return
	}
}
