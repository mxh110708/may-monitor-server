// Local build helper: pack the tested frontend into the server embed archive.
package main

import (
	"archive/tar"
	"github.com/klauspost/compress/zstd"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func main() {
	source, output := os.Args[1], os.Args[2]
	must(os.MkdirAll(filepath.Dir(output), 0755))
	f, err := os.Create(output)
	must(err)
	z, err := zstd.NewWriter(f)
	must(err)
	t := tar.NewWriter(z)
	must(filepath.WalkDir(source, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(source, p)
		if err != nil {
			return err
		}
		must(t.WriteHeader(&tar.Header{Name: filepath.ToSlash(name), Mode: 0644, Size: info.Size()}))
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(t, in)
		return err
	}))
	must(t.Close())
	must(z.Close())
	must(f.Close())
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
