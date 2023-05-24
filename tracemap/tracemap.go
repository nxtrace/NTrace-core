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
	resp, err := http.Post(url, "application/json", strings.NewReader(r))
	if err != nil {
		fmt.Println("An issue occurred while connecting to the tracemap API.")
		return
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("An issue occurred while connecting to the tracemap API.")
		return
	}
	fmt.Fprintf(color.Output, "%s %s\n",
		color.New(color.FgWhite, color.Bold).Sprintf("%s", "MapTrace URL:"),
		color.New(color.FgBlue, color.Bold).Sprintf("%s", string(body)),
	)
}
