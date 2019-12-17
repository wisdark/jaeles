package core

import (
	"archive/zip"
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jaeles-project/jaeles/database"
	"github.com/mitchellh/go-homedir"
)

// GetFileContent Reading file and return content of it
func GetFileContent(filename string) string {
	var result string
	if strings.Contains(filename, "~") {
		filename, _ = homedir.Expand(filename)
	}
	file, err := os.Open(filename)
	if err != nil {
		return result
	}
	defer file.Close()
	b, err := ioutil.ReadAll(file)
	if err != nil {
		return result
	}
	return string(b)
}

// ReadingFile Reading file and return content as []string
func ReadingFile(filename string) []string {
	var result []string
	if strings.HasPrefix(filename, "~") {
		filename, _ = homedir.Expand(filename)
	}
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return result
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		val := scanner.Text()
		result = append(result, val)
	}

	if err := scanner.Err(); err != nil {
		return result
	}
	return result
}

// ReadingFileUnique Reading file and return content as []string
func ReadingFileUnique(filename string) []string {
	var result []string
	if strings.Contains(filename, "~") {
		filename, _ = homedir.Expand(filename)
	}
	file, err := os.Open(filename)
	defer file.Close()
	if err != nil {
		return result
	}

	unique := true
	seen := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		val := scanner.Text()
		// unique stuff
		if val == "" {
			continue
		}
		if seen[val] && unique {
			continue
		}

		if unique {
			seen[val] = true
			result = append(result, val)
		}
	}

	if err := scanner.Err(); err != nil {
		return result
	}
	return result
}

// WriteToFile write string to a file
func WriteToFile(filename string, data string) (string, error) {
	file, err := os.Create(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.WriteString(file, data)
	if err != nil {
		return "", err
	}
	return filename, file.Sync()
}

// FileExists check if file is exist or not
func FileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// FolderExists check if file is exist or not
func FolderExists(foldername string) bool {
	if _, err := os.Stat(foldername); os.IsNotExist(err) {
		return false
	}
	return true
}

// GetFileNames get all file name with extension
func GetFileNames(dir string, ext string) []string {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	var files []string
	filepath.Walk(dir, func(path string, f os.FileInfo, _ error) error {
		if !f.IsDir() {
			if strings.HasSuffix(f.Name(), ext) {
				filename, _ := filepath.Abs(path)
				files = append(files, filename)
			}
		}
		return nil
	})
	return files
}

// IsJSON check if string is JSON or not
func IsJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

// GetTS get current timestamp and return a string
func GetTS() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// GenHash gen SHA1 hash from string
func GenHash(text string) string {
	h := sha1.New()
	h.Write([]byte(text))
	hashed := h.Sum(nil)
	return fmt.Sprintf("%x", hashed)
}

// Unzip will decompress a zip archive, moving all files and folders
// within the zip file (parameter 1) to an output directory (parameter 2).
func Unzip(src string, dest string) ([]string, error) {

	var filenames []string

	r, err := zip.OpenReader(src)
	if err != nil {
		return filenames, err
	}
	defer r.Close()

	for _, f := range r.File {

		// Store filename/path for returning and using later on
		fpath := filepath.Join(dest, f.Name)

		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return filenames, fmt.Errorf("%s: illegal file path", fpath)
		}

		filenames = append(filenames, fpath)

		if f.FileInfo().IsDir() {
			// Make Folder
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Make File
		if err = os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return filenames, err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return filenames, err
		}

		rc, err := f.Open()
		if err != nil {
			return filenames, err
		}

		_, err = io.Copy(outFile, rc)

		// Close the file without defer to close before next iteration of loop
		outFile.Close()
		rc.Close()

		if err != nil {
			return filenames, err
		}
	}
	return filenames, nil
}

// ExpandLength make slice to length
func ExpandLength(list []string, length int) []string {
	c := []string{}
	for i := 1; i <= length; i++ {
		c = append(c, list[i%len(list)])
	}
	return c
}

// SelectSign select signature by multiple selector
func SelectSign(signName string) []string {
	var Signs []string

	// return default sign if doesn't set anything
	if signName == "" {
		Signs = database.SelectSign(signName)
	}

	if strings.Contains(signName, ",") {
		rawSigns := strings.Split(signName, ",")
		for _, rawSign := range rawSigns {
			signs := SingleSign(strings.TrimSpace(rawSign))
			if len(signs) > 0 {
				Signs = append(Signs, signs...)
			}
		}
	} else {
		signs := SingleSign(strings.TrimSpace(signName))
		if len(signs) > 0 {
			Signs = append(Signs, signs...)
		}
	}

	return Signs
}

// SingleSign select signature by single selector
func SingleSign(signName string) []string {
	if strings.HasPrefix(signName, "~") {
		signName, _ = homedir.Expand(signName)
	}

	var Signs []string
	if strings.HasSuffix(signName, ".yaml") {
		if FileExists(signName) {
			Signs = append(Signs, signName)
		}
	}
	// get more sign nature
	if strings.Contains(signName, "*") && strings.Contains(signName, "/") {
		asbPath, _ := filepath.Abs(signName)
		baseSelect := filepath.Base(signName)
		rawSigns := GetFileNames(filepath.Dir(asbPath), "yaml")
		for _, signFile := range rawSigns {
			baseSign := filepath.Base(signFile)
			if len(baseSign) == 1 && baseSign == "*" {
				Signs = append(Signs, signFile)
				continue
			}
			r, err := regexp.Compile(baseSelect)
			if err != nil {
				continue
			}
			if r.MatchString(baseSign) {
				Signs = append(Signs, signFile)
			}
		}
	}
	return Signs
}
