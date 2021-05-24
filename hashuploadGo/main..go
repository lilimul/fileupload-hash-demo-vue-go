package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

var temp_dir = "./chunks"

type mergeReq struct {
	FileName string
	Hash     string
}
type JsonResult struct {
	Code int
	Msg  string
}
type FileVerfyJsonResult struct {
	JsonResult
	ShouldUpload      bool
	UploadedChunkIdxs []int
}
type chunkFile struct {
	FileName string
	chunkIdx int
}
type verfyFileReq struct {
	FileName string
	Hash     string
}

func main() {
	http.HandleFunc("/upload", uploadHandle)
	http.HandleFunc("/merge", mergeHandle)
	http.HandleFunc("/verify", verfyFile)
	err := http.ListenAndServe("localhost:8081", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}

func getChunkFiles(pathname string, targetFileHash string) ([]chunkFile, error) {
	chunks := []chunkFile{}
	filepath.Walk(pathname, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		fmt.Println(info.Name())
		if fileNameArr := strings.FieldsFunc(info.Name(), func(c rune) bool { return c == '-' }); fileNameArr[0] == targetFileHash {
			if len(fileNameArr) != 2 {
				return nil
			}
			chunkIdx, err := strconv.Atoi(fileNameArr[1])
			if err != nil {
				return nil
			}
			chunks = append(chunks, chunkFile{path, chunkIdx})
		}
		return nil
	})
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].chunkIdx < chunks[j].chunkIdx
	})
	return chunks, nil
}
func mergeFileChunks(chunks []chunkFile, fileName string, fileHash string, baseFolder string) {
	if len(chunks) == 0 {
		return
	}
	hashedFileName := fileHash + "-" + fileName
	mergeFilePath := path.Join("./", baseFolder, hashedFileName)

	fii, err := os.OpenFile(mergeFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	defer fii.Close()
	if err != nil {
		//panic(err)
		return
	}
	for _, file := range chunks {
		f, err := os.OpenFile(file.FileName, os.O_RDONLY, os.ModePerm)
		if err != nil {
			fmt.Println(err)
			return
		}
		b, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Println(err)
			return
		}
		fii.Write(b)
		f.Close()
	}
}
func init() {
	_, err := os.Stat(temp_dir)
	if err != nil {
		fmt.Println("stat temp dir error,maybe is not exist, maybe not")
		if os.IsNotExist(err) {
			fmt.Println("temp dir is not exist")
			err := os.Mkdir(temp_dir, os.ModePerm)
			if err != nil {
				fmt.Printf("mkdir failed![%v]\n", err)
			}
			return
		}

		fmt.Println("stat file error")
		return
	}
	return
}
func uploadHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("content-type", "application/json")
	formFile, _, err := r.FormFile("chunk")
	fileName := r.FormValue("fileName")
	fmt.Println("Now uploading chunk of ", fileName)
	hash := r.FormValue("hash")
	if r.Method != "POST" {
		msg, _ := json.Marshal(JsonResult{Code: 503, Msg: "请以POST方式上传"})
		w.Write(msg)
		return
	}
	if err != nil {
		log.Printf("Get form file failed: %s\n", err)
		return
	}
	defer formFile.Close()
	destFile, err := os.Create(path.Join("./chunks", hash))
	if err != nil {
		log.Printf("Create failed: %s\n", err)
		return
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, formFile)
	if err != nil {
		log.Printf("Write file failed: %s\n", err)
		return
	}
	msg, _ := json.Marshal(JsonResult{Code: 200, Msg: "分片成功"})
	w.Write(msg)
	return
}
func removeChunks(chunks []chunkFile) {
	for _, file := range chunks {
		err := os.Remove(file.FileName)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
}
func mergeHandle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("content-type", "application/json")
	var mergereq mergeReq
	reqParseErr := json.NewDecoder(r.Body).Decode(&mergereq)
	if reqParseErr != nil || mergereq.Hash == "" {
		msg, _ := json.Marshal(JsonResult{Code: 501, Msg: "Unsupported merge request"})
		w.Write(msg)
		return
	}
	targetFileName := mergereq.FileName
	targetFileHash := mergereq.Hash
	fileChunks, _ := getChunkFiles(temp_dir, targetFileHash)
	if len(fileChunks) <= 0 {
		msg, _ := json.Marshal(JsonResult{Code: 404, Msg: "File chunks not found"})
		w.Write(msg)
		return
	}
	mergeFileChunks(fileChunks, targetFileName, targetFileHash, "merge")
	msg, _ := json.Marshal(JsonResult{Code: 200, Msg: "File chunks merged"})
	removeChunks(fileChunks)
	w.Write(msg)
	return
}
func verfyFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("content-type", "application/json")
	var verfyFilereq verfyFileReq
	reqParseErr := json.NewDecoder(r.Body).Decode(&verfyFilereq)
	if reqParseErr != nil {
		msg, _ := json.Marshal(FileVerfyJsonResult{JsonResult: JsonResult{Code: 501, Msg: "Veryfy failed"}, ShouldUpload: true, UploadedChunkIdxs: []int{}})
		fmt.Print(string(msg))
		w.Write(msg)
		return
	}
	reqHash := verfyFilereq.Hash
	pathname := "./merge"
	err := filepath.Walk(pathname, func(filePath string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		fileName := path.Base(info.Name())
		fileNameArr := strings.FieldsFunc(fileName, func(c rune) bool { return c == '-' })
		if len(fileNameArr) != 2 {
			return nil
		}
		fileHash := fileNameArr[0]
		if reqHash == fileHash {
			newa := FileVerfyJsonResult{ShouldUpload: false, UploadedChunkIdxs: []int{}, JsonResult: JsonResult{Code: 200, Msg: "File hash matched"}}
			msg, _ := json.Marshal(newa)
			fmt.Println(newa)
			fmt.Println(string(msg))
			w.Write(msg)
			return io.EOF
		}
		return nil
	})
	if err == io.EOF { //find file!
		return
	}
	//find chunks
	matchedChunks := []int{}
	fileChunks, _ := getChunkFiles(temp_dir, reqHash)
	for _, uploadedChunk := range fileChunks {
		matchedChunks = append(matchedChunks, uploadedChunk.chunkIdx)
	}
	msg, _ := json.Marshal(FileVerfyJsonResult{JsonResult: JsonResult{Code: 200, Msg: "File hash not matched"}, ShouldUpload: true, UploadedChunkIdxs: matchedChunks})
	w.Write(msg)
	return
}
