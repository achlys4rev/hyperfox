/*
	Written by José Carlos Nieto <xiam@menteslibres.org>
	License MIT
*/

package save

import (
	"github.com/xiam/hyperfox/proxy"
	"io"
	"net/http"
	"path"
	"os"
)

func Body(res *http.Response) io.WriteCloser {

	file := proxy.ArchiveFile(res)

	proxy.Workdir(path.Dir(file))

	fp, _ := os.Create(file)

	return fp
}

func Head(res *http.Response) io.WriteCloser {

	file := proxy.ArchiveFile(res) + ".head"

	proxy.Workdir(path.Dir(file))

	fp, _ := os.Create(file)

	if fp != nil {
		res.Header.Write(fp)
		fp.Close()
	}

	return nil
}
