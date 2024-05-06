package sensitivity_labels

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ExitError(e error) {
	fmt.Println(e.Error())
	os.Exit(1)
}

func templateLabelInfoXml(labels Labels) string {
	xmlStr := `<?xml version="1.0" encoding="utf-8" standalone="yes"?>`
	xmlStr += `<clbl:labelList xmlns:clbl="http://schemas.microsoft.com/office/2020/mipLabelMetadata">`
	for _, label := range labels.Labels {
		xmlStr += fmt.Sprintf(
			`<clbl:label id="%s" enabled="1" method="Privileged" siteId="%s" contentBits="0" removed="0"/>`,
			label.Id,
			label.SiteId,
		)
	}
	xmlStr += `</clbl:labelList>`
	return xmlStr
}

func SetLabelInfoXml(filePath string, labels Labels) {
	err := os.WriteFile(filePath, []byte(templateLabelInfoXml(labels)), 0644)
	if err != nil {
		fmt.Println("warn: error writing " + filePath)
	}
}

func GetLabelInfoXml(filePath string) Labels {
	var labels Labels
	xmlFile, err := os.Open(filePath)
	if err != nil {
		fmt.Println(err)
	}
	defer xmlFile.Close()
	byteValue, _ := io.ReadAll(xmlFile)
	xml.Unmarshal(byteValue, &labels)
	return labels
}

func CheckLabelInfoPath(dirPath string) (bool, string) {
	labelInfoPath := dirPath + "/docMetadata/labelInfo.xml"
	_, err := os.Stat(labelInfoPath)
	return (err == nil), labelInfoPath
}

func Unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
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

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
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

func isExtensionFile(file os.FileInfo, exts []string) bool {
	for _, ext := range exts {
		if filepath.Ext(file.Name()) == ext {
			return true
		}
	}
	return false
}

func filterFilesByExtension(files []os.FileInfo, exts []string) []os.FileInfo {
	var filteredFiles []os.FileInfo
	for _, file := range files {
		if isExtensionFile(file, exts) {
			filteredFiles = append(filteredFiles, file)
		}
	}
	return filteredFiles
}

func ListExtensionFiles(dir string, recursive bool, exts []string) []os.FileInfo {
	var files []fs.FileInfo

	if !recursive {
		items, err := os.ReadDir(dir)
		if err != nil {
			ExitError(err)
		}

		for _, item := range items {
			if !item.IsDir() {
				info, err := item.Info()
				if err != nil {
					ExitError(err)
				}
				files = append(files, info)
			}
		}

	} else {
		// recursively list files
		err := filepath.Walk(dir,
			func(path string, info os.FileInfo, err error) error {
				if err != nil {
					ExitError(err)
				}
				if !info.IsDir() {
					files = append(files, info)
				}
				return nil
			})
		if err != nil {
			ExitError(err)
		}
	}
	return filterFilesByExtension(files, exts)
}
