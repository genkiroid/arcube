package arcube

import (
	"archive/zip"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	cwd string
	wd  = "/tmp/arcube"
	exd string
)

func init() {
	var err error
	cwd, err = os.Getwd()
	if err != nil {
		log.Fatalf("os.Getwd failed: %v", err)
	}
}

func Run(zipPath string) error {
	clean()

	zipName := filepath.Base(zipPath)
	dir := strings.Replace(zipName, ".zip", "", -1)
	exd = filepath.Join(wd, dir)

	if err := Unzip(zipPath); err != nil {
		return err
	}

	if err := MoveFiles(); err != nil {
		return err
	}

	if err := ReplaceFiles(); err != nil {
		return err
	}

	if err := ModifyFiles(); err != nil {
		return err
	}

	if err := RemoveFiles(); err != nil {
		return err
	}

	if err := Zip(zipName); err != nil {
		return err
	}

	return nil
}

func clean() error {
	_, err := os.Stat(wd)
	if err == nil {
		err := os.RemoveAll(wd)
		if err != nil {
			return err
		}
	}

	return nil
}

func Unzip(zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(wd, 0755)

	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(wd, f.Name)

		switch mode := f.FileInfo().Mode(); {
		case mode.IsDir():
			os.MkdirAll(path, f.Mode())
		case mode&os.ModeSymlink != 0:
			buffer := make([]byte, f.FileInfo().Size())
			size, err := rc.Read(buffer)
			if err != nil {
				return err
			}

			target := string(buffer[:size])

			err = os.Symlink(target, path)
			if err != nil {
				return err
			}
		case mode.IsRegular():
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err = f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}

func MoveFiles() error {
	maps := []*RenameMap{
		&RenameMap{"html/index.php", "index.php"},
		&RenameMap{"html/index_dev.php", "index_dev.php"},
		&RenameMap{"html/install.php", "install.php"},
		&RenameMap{"html/robots.txt", "robots.txt"},
		&RenameMap{"html/.htaccess", ".htaccess"},
		&RenameMap{"html/web.config", "web.config"},
	}

	for _, m := range maps {
		err := os.Rename(filepath.Join(exd, m.old), filepath.Join(exd, m.new))
		if err != nil {
			return err
		}
	}

	return nil
}

func ReplaceFiles() error {
	if err := os.Remove(filepath.Join(exd, ".htaccess")); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(exd, "web.config")); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(exd, ".htaccess.sample"), filepath.Join(exd, ".htaccess")); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(exd, "web.config.sample"), filepath.Join(exd, "web.config")); err != nil {
		return err
	}
	return nil
}

func ModifyFiles() error {
	maps := []*ReplaceMap{
		&ReplaceMap{
			filepath.Join(exd, "index.php"),
			"require __DIR__.'/../autoload.php';",
			"//require __DIR__.'/../autoload.php';",
		},
		&ReplaceMap{
			filepath.Join(exd, "index.php"),
			"//require __DIR__.'/autoload.php';",
			"require __DIR__.'/autoload.php';",
		},
		&ReplaceMap{
			filepath.Join(exd, "index_dev.php"),
			"require_once __DIR__.'/../autoload.php';",
			"//require_once __DIR__.'/../autoload.php';",
		},
		&ReplaceMap{
			filepath.Join(exd, "index_dev.php"),
			"//require_once __DIR__.'/autoload.php';",
			"require_once __DIR__.'/autoload.php';",
		},
		&ReplaceMap{
			filepath.Join(exd, "index_dev.php"),
			"'profiler.cache_dir' => __DIR__.'/../app/cache/profiler',",
			"'profiler.cache_dir' => __DIR__.'/app/cache/profiler',",
		},
		&ReplaceMap{
			filepath.Join(exd, "install.php"),
			"require __DIR__ . '/../autoload.php';",
			"//require __DIR__ . '/../autoload.php';",
		},
		&ReplaceMap{
			filepath.Join(exd, "install.php"),
			"//require __DIR__ . '/autoload.php';",
			"require __DIR__ . '/autoload.php';",
		},
		&ReplaceMap{
			filepath.Join(exd, "autoload.php"),
			`define("RELATIVE_PUBLIC_DIR_PATH", '');`,
			`//define("RELATIVE_PUBLIC_DIR_PATH", '');`,
		},
		&ReplaceMap{
			filepath.Join(exd, "autoload.php"),
			`//define("RELATIVE_PUBLIC_DIR_PATH", '/html');`,
			`define("RELATIVE_PUBLIC_DIR_PATH", '/html');`,
		},
	}

	for _, m := range maps {
		if err := m.replace(); err != nil {
			return err
		}
	}

	return nil
}

func RemoveFiles() error {
	if err := os.Remove(filepath.Join(exd, "eccube_install.sh")); err != nil {
		return err
	}
	return nil
}

func Zip(zipName string) error {
	out, err := filepath.Abs(filepath.Join(cwd, zipName))
	if err != nil {
		return err
	}

	z, err := os.Create(out)
	if err != nil {
		return err
	}
	defer func() {
		log.Printf("created %s", out)
		z.Close()
	}()

	w := zip.NewWriter(z)
	defer w.Close()

	os.Chdir(wd)

	err = filepath.Walk(filepath.Base(exd), func(path string, info os.FileInfo, err error) error {
		header, err := makeHeader(w, path, info)
		if err != nil {
			return err
		}

		switch mode := info.Mode(); {
		case mode.IsDir():
			// noop
		case mode.IsRegular():
			b, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			r := bytes.NewReader(b)

			_, err = io.Copy(header, r)
			if err != nil {
				return err
			}
		case mode&os.ModeSymlink != 0:
			s, err := os.Readlink(path)
			if err != nil {
				return err
			}

			r := strings.NewReader(s)

			_, err = io.Copy(header, r)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func makeHeader(w *zip.Writer, path string, info os.FileInfo) (io.Writer, error) {
	fi, err := zip.FileInfoHeader(info)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		fi.Name = path + "/"
		fi.Method = zip.Store
	} else {
		fi.Name = path
		fi.Method = zip.Deflate
	}

	local := time.Now().Local()
	_, offset := local.Zone()
	fi.SetModTime(fi.ModTime().Add(time.Duration(offset) * time.Second))

	header, err := w.CreateHeader(fi)
	if err != nil {
		return nil, err
	}

	return header, nil
}

type RenameMap struct {
	old string
	new string
}

type ReplaceMap struct {
	path string
	old  string
	new  string
}

func (m *ReplaceMap) replace() error {
	c, err := ioutil.ReadFile(m.path)
	if err != nil {
		return err
	}
	s := string(c)
	s = strings.Replace(s, m.old, m.new, 1)

	f, err := os.OpenFile(m.path, os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(f, strings.NewReader(s))
	if err != nil {
		return err
	}

	return nil
}
