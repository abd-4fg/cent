package jobs

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	filepath "path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/go-git/go-git/v5"
	"github.com/spf13/viper"
	"github.com/abd-4fg/cent/internal/utils"
)

func cloneRepo(gitPath string, console bool, index string, timestamp string) error {
	cloneOptions := &git.CloneOptions{
		URL: gitPath,
	}

	destDir := filepath.Join(os.TempDir(), fmt.Sprintf("cent%s/repo%s", timestamp, index))

	_, err := git.PlainClone(destDir, false, cloneOptions)
	if err != nil {
		return err
	}

	fmt.Printf(color.GreenString("[CLONED] %s\n", gitPath))
	return nil
}

func worker(work chan [2]string, wg *sync.WaitGroup, console bool, timestamp string, defaultTimeout int) {
	defer wg.Done()
	for repo := range work {
		err := cloneRepo(repo[1], console, repo[0], timestamp)
		if err != nil {
			fmt.Println(color.RedString("[ERR] clone: " + repo[1] + " - " + err.Error()))
		}
	}
}

func Start(_path string, console bool, threads int, defaultTimeout int) {
	timestamp := strconv.Itoa(int(time.Now().Unix()))
	if _, err := os.Stat(filepath.Join(_path)); os.IsNotExist(err) {
		os.Mkdir(filepath.Join(_path), 0700)
	}

	work := make(chan [2]string)
	go func() {
		for index, gitPath := range viper.GetStringSlice("community-templates") {
			work <- [2]string{strconv.Itoa(index), gitPath}
		}
		close(work)
	}()

	wg := &sync.WaitGroup{}

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go worker(work, wg, console, timestamp, defaultTimeout)
	}
	wg.Wait()

	dirname := filepath.Join(os.TempDir(), "cent"+timestamp)

	filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		directory := getDirPath(strings.TrimPrefix(path, dirname))

		if info.IsDir() {
		} else {
			basename := info.Name()
			if filepath.Ext(basename) == ".yaml" {
				directory = ""
				sourcePath := path
				destinationPath := filepath.Join(_path, directory, basename)

				err := utils.CopyFile(sourcePath, destinationPath)
				if err != nil {
					return err
				}
			}
		}

		return nil
	})

	DeleteFromTmp(dirname)
}

func UpdateRepo(path string, remDirs bool, remFiles bool, printOut bool) {
	filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				if remDirs {
					for _, exDirs := range viper.GetStringSlice("exclude-dirs") {
						if strings.Contains(path, exDirs) {
							err := os.RemoveAll(path)
							if err != nil {
								log.Fatal(err)
							}
							if printOut {
								fmt.Println(color.RedString("[D][-] Dir  removed\t" + path))
							}
							return filepath.SkipDir
						}
					}
				}
			} else {
				if remFiles {
					for _, exFiles := range viper.GetStringSlice("exclude-files") {
						// fmt.Println("Path: ", path, exFiles)
						if strings.Contains(path, exFiles) {
							e := os.Remove(path)
							if e != nil {
								log.Fatal(e)
							}
							if printOut {
								fmt.Println(color.RedString("[F][-] File removed\t" + path))
							}

						}
						// break
					}
				}
			}
			return nil
		})
}


func getFilePaths(path string) []string {
	var files []string

	// go through each file
	err := filepath.WalkDir(path, func(s string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if !d.IsDir() {
			files = append(files, s)
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
	return files
}

func getFileHash(path string) string {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		log.Fatal(err)
	}

	return fmt.Sprintf("%x", h.Sum(nil))
}

func getDirPath(path string) string {
	reponame := strings.Split(path, "/")[0]
	endpoint := strings.TrimPrefix(path, reponame)
	return endpoint
}

func RemoveEmptyFolders(dirname string) {

	f, err := os.Open(dirname)
	if err != nil {
		log.Fatal(err)
	}
	files, err := f.Readdir(-1)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		if file.IsDir() {
			if IsEmpty(filepath.Join(dirname, file.Name())) {
				err := os.RemoveAll(filepath.Join(dirname, file.Name()))
				if err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func IsEmpty(name string) bool {
	f, err := os.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)

	return err == io.EOF // Either not empty or error, suits both cases
}

func DeleteFromTmp(dirname string) {
	err := os.RemoveAll(dirname)
	if err != nil {
		log.Fatal(err)
	}
}
