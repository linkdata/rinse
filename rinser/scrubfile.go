package rinser

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func scrub(dstpath string) error {
	var errs []error
	err := filepath.Walk(dstpath, func(fpath string, info fs.FileInfo, err error) error {
		if err == nil {
			if !info.IsDir() {
				var f *os.File
				if f, err = os.OpenFile(fpath, os.O_RDWR, 0666); err == nil {
					fourk := make([]byte, 4096)
					remain := info.Size()
					for err == nil && remain > 0 {
						n := len(fourk)
						if remain < int64(n) {
							n = int(remain)
						}
						if n, err = f.Write(fourk[:n]); err == nil {
							remain -= int64(n)
						}
					}
					errs = append(errs, f.Close())
				}
				errs = append(errs, err)
			}
		}
		return nil
	})
	if !errors.Is(err, fs.ErrNotExist) {
		errs = append(errs, err)
	}
	errs = append(errs, os.RemoveAll(dstpath))
	return errors.Join(errs...)
}
