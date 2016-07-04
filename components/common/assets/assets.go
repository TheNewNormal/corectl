package assets

import (
	"io/ioutil"
	"net/http"

	"github.com/helm/helm/log"
)

func Contents(file string) (t string) {
	var (
		err error
		buf []byte
		f   http.File
	)
	if f, err = Assets.Open(file); err != nil {
		log.Err(err.Error())
		return
	}
	defer f.Close()
	if buf, err = ioutil.ReadAll(f); err != nil {
		log.Err(err.Error())
		return
	}
	return string(buf)
}
