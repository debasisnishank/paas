package main

import (
	"net/http"
	"os"
	"time"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<h1>Hello from a BUILT Antariksh app</h1>\n"))
	})
	os.Stdout.WriteString("ANTARIKSH_BUILT_APP_UP\n")
	for {
		if err := http.ListenAndServe(":80", nil); err != nil {
			os.Stdout.WriteString("listen err: " + err.Error() + "\n")
			time.Sleep(time.Second)
		}
	}
}
