package main

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var tempPath = filepath.Join(os.Getenv("TEMP"), "TEMP")

type FileInfo struct {
	FilePath  string // file location
	Content   []byte // file content
	Sum       string // md5 sum of content
	IsRenamed bool   // indicator whether the particular file is renamed already or not
}

func main() {
	log.Println("Start Program")

	start := time.Now()

	// pipeline 1 : loop all files and read it
	chanFileContent := readFiles()

	// pipeline2 : calculate md5sum, metode Fan-Out-Function (menggunakan banyak worker, lalu dimerge)
	chanFileSum1 := getSum(chanFileContent)
	chanFileSum2 := getSum(chanFileContent)
	chanFileSum3 := getSum(chanFileContent)
	chanFileSum := mergeChanFileInfo(chanFileSum1, chanFileSum2, chanFileSum3)

	// pipeline 3 : rename files
	chanRename1 := rename(chanFileSum)
	chanRename2 := rename(chanFileSum)
	chanRename3 := rename(chanFileSum)
	chanRename4 := rename(chanFileSum)
	chanRename := mergeChanFileInfo(chanRename1, chanRename2, chanRename3, chanRename4)

	// print output
	counterRenamed := 0
	counterTotal := 0
	for fileInfo := range chanRename {
		if fileInfo.IsRenamed {
			counterRenamed++
		}
		counterTotal++
	}
	log.Printf("%d/%d files renamed", counterRenamed, counterTotal)
	duration := time.Since(start)
	log.Println("done in", duration.Seconds(), "seconds")
}

func rename(chanIn <-chan FileInfo) <-chan FileInfo {
	chanOut := make(chan FileInfo)

	go func() {
		for fileInfo := range chanIn {
			newPath := filepath.Join(tempPath, fmt.Sprintf("file-%s.txt", fileInfo.Sum))

			err := os.Rename(fileInfo.FilePath, newPath)

			if err != nil {
				log.Println("Error :", err.Error())
			}

			fileInfo.IsRenamed = err == nil
			chanOut <- fileInfo
		}
		close(chanOut)
	}()
	return chanOut
}

func readFiles() <-chan FileInfo { // mengirim data melalui channel
	chanOut := make(chan FileInfo)

	go func() {
		err := filepath.Walk(tempPath, func(path string, info os.FileInfo, err error) error {

			// jika ada error
			if err != nil {
				return err
			}

			// jika yang didapat subdirektori/subfolder
			if info.IsDir() {
				return nil
			}

			buf, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}

			// FileInfo mengirim data ke chanOut
			chanOut <- FileInfo{
				FilePath: path,
				Content:  buf,
			}

			return nil
		})

		if err != nil {
			log.Println("Error :", err.Error())
		}

		close(chanOut) // menutup koneksi channel
	}()
	return chanOut
}

func getSum(chanIn <-chan FileInfo) <-chan FileInfo { // menerima data, return mengirimkan data
	chanOut := make(chan FileInfo)

	go func() {
		for fileInfo := range chanIn {
			fileInfo.Sum = fmt.Sprintf("%x", md5.Sum(fileInfo.Content))
			chanOut <- fileInfo
		}
		close(chanOut)
	}()
	return chanOut
}

func mergeChanFileInfo(chanInMany ...<-chan FileInfo) <-chan FileInfo {
	wg := new(sync.WaitGroup)
	chanOut := make(chan FileInfo)

	wg.Add(len(chanInMany))
	for _, eachChan := range chanInMany {
		go func(eachChan <-chan FileInfo) {
			for eachChanData := range eachChan {
				chanOut <- eachChanData
			}
			wg.Done()
		}(eachChan) // mengembalikan eachChan
	}

	go func() {
		wg.Wait()
		close(chanOut)
	}()
	return chanOut
}
