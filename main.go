// is project main.go
package main

import (
	"bytes"
	"crypto/md5"
	"flag"
	"runtime"

	"image"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/sidneychen/imaging"
)

var (
	MEMCACHE_ADDR = "192.168.94.26:11211"
	FORMAT        = imaging.JPEG
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	cfgPath := flag.String("conf", "./config.json", "config file path")
	flag.Parse()

	cfg := NewConfigFromFile(*cfgPath)
	MEMCACHE_ADDR = cfg.cc.Addr

	r := mux.NewRouter()
	r.HandleFunc("/favicon.ico", nilHandler).Methods("GET")
	r.HandleFunc("/{mode}/{pid:[a-z0-9]+}", getImage).Methods("GET")
	r.HandleFunc("/upload", uploadImage).Methods("POST")

	log.Println("Server started and listening on ", cfg.Listen)
	err := http.ListenAndServe(cfg.Listen, r)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}

func nilHandler(w http.ResponseWriter, r *http.Request) {
	return
}

func getKey(pid, mode string) string {
	return "image_" + pid + "_" + mode
}

func getImage(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	mode := params["mode"]
	pid := params["pid"]

	cache := NewMemcache(MEMCACHE_ADDR)

	flush := r.URL.Query().Get("flush")
	if flush == "1" {
		cache.Delete(getKey(pid, mode))
		log.Println("Delete image from cache: ", pid, mode)
	}

	var data []byte
	var err error

	data, err = cache.Get(getKey(pid, mode))
	if err == nil {
		log.Println("Get image from cache: ", pid, mode)
		w.Write(data)
		return
	} else {
		log.Println("Get from cache fail pid=", pid, err)
	}

	log.Println("Get image from fs: ", pid, mode)
	file, err := getImageFromPid(pid)
	if err != nil {
		log.Fatal("ReadAll: ", err.Error())
		return
	}
	defer file.Close()

	img, err := imaging.Decode(file)
	if err != nil {
		log.Fatal("Image Decode: ", err.Error())
	}
	dstImage := transformImage(img, mode)

	var buf []byte
	buffer := bytes.NewBuffer(buf)
	imaging.Encode(buffer, dstImage, FORMAT, 50)
	data, err = ioutil.ReadAll(buffer)
	if err != nil {
		log.Fatal("ReadAll: ", err.Error())
		return
	}
	cache.Set(getKey(pid, mode), data, 86400)
	w.Write(data)
}

func transformImage(img image.Image, mode string) image.Image {
	if mode == "large" {
		return img
	} else if mode == "small" {
		return resize(img, 128)
	} else if mode == "thumbnail" {
		return resize(img, 80)
	}
	return img
}

func uploadImage(w http.ResponseWriter, r *http.Request) {

	log.Print("Start upload image...")
	file, _, err := r.FormFile("image")
	if err != nil {
		log.Panicln("from file: ", err.Error())
		return
	}
	defer file.Close()

	img, err := imaging.Decode(file)
	if err != nil {
		log.Panicln("Decode: ", err.Error())
		return
	}

	// image metadata
	width := img.Bounds().Max.X
	height := img.Bounds().Max.Y
	file.Seek(0, 0)
	fileContent, err := ioutil.ReadAll(file)
	length := len(fileContent)
	hashValue := md5.Sum(fileContent)

	pid := getPictureId(length, width, height, hashValue)

	dstImageThumb := resize(img, 1034)
	filename, err := getFilename(pid)

	err = saveToFilesystem(dstImageThumb, filename)
	if err != nil {
		log.Panicln("Save: ", err.Error())
		return
	}

	w.Write([]byte(`{"pid":"` + pid + `"}`))

}

func getPictureId(length, width, height int, hashValue [16]byte) string {
	return convert10To36(uint64(length), 5) + convert10To36(uint64(width), 3) + convert10To36(uint64(height), 3) + convertBytesTo36(hashValue) + convert10To36(uint64(imaging.JPEG), 1)
}

func convert10To36(val uint64, length int) string {
	strVal := strconv.FormatUint(uint64(val), 36)
	strLength := len(strVal)
	for strLength < length {
		strVal = "0" + strVal
		strLength++
	}
	return strVal
}

func convertBytesTo36(bytes [16]byte) string {
	var first, second uint64 = 0, 0
	for i, val := range bytes[:8] {
		first += uint64(val) << uint(8*i)
	}
	for i, val := range bytes[8:] {
		second += uint64(val) << uint(8*i)
	}
	return convert10To36(first, 13) + convert10To36(second, 13)
}

func resize(img image.Image, width int) image.Image {
	if img.Bounds().Max.X < width {
		width = img.Bounds().Max.X
	}
	dstImage := imaging.Resize(img, width, 0, imaging.NearestNeighbor)
	//dstImage = imaging.Sharpen(dstImage, 1)
	return dstImage
}

func getFilename(pid string) (string, error) {
	root := "./photos"
	first := pid[11:14]
	second := pid[14:17]
	dirname := path.Join(root, first, second)
	if err := os.MkdirAll(dirname, 0755); err != nil {
		log.Println("Create dir fail", err.Error())
		return "", err
	}
	log.Println("Create dir", dirname)
	return path.Join(dirname, pid), nil
}

func saveToFilesystem(img image.Image, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	return imaging.Encode(file, img, FORMAT, 75)
}

func getImageFromPid(pid string) (*os.File, error) {
	filename, _ := getFilename(pid)
	return os.Open(filename)

}
