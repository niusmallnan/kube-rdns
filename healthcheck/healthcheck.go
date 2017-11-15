package healthcheck

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

func StartHealthCheck(listen string) error {
	http.HandleFunc("/healthcheck", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	logrus.Infof("Listening for health checks on 0.0.0.0:%d/healthcheck", listen)
	err := http.ListenAndServe(listen, nil)
	return err
}
