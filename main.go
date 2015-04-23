// is project main.go
package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	//	"github.com/kklis/gomemcache"
	"github.com/bradfitz/gomemcache/memcache"
	"image"
	"imaging"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	MEMCACHE_ADDR string = "192.168.94.26:11211"
)

func main() {
	fmt.Println("Start")
	http.HandleFunc("/upload", uploadImage)
	http.HandleFunc("/", getImage)
	http.HandleFunc("/favicon.ico", nilFunc)
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err.Error())
	}
}

func nilFunc(w http.ResponseWriter, r *http.Request) {
	return
}

func getKey(pid, mode string) string {
	return "image_" + pid + "_" + mode
}

func getImage(w http.ResponseWriter, r *http.Request) {
	segments := strings.Split(r.URL.Path, "/")
	if len(segments) != 3 {
		fmt.Fprint(w, "invalid uri, ", r.URL)
		log.Println("invalid uri, ", r.URL)
		return
	}

	mode := segments[1]
	pid := segments[2]

	memc := memcache.New(MEMCACHE_ADDR)
	memc.Set(&memcache.Item{Key: "foo", Value: []byte("my value")})
	flush := r.URL.Query().Get("flush")
	if flush == "1" {
		memc.Delete(getKey(pid, mode))
		log.Println("Delete Image From Memcache: ", pid, mode)
	}

	item, err := memc.Get(getKey(pid, mode))
	if err == nil {
		log.Println("Get Image From Memcache: ", pid, mode)
		w.Write(item.Value)
		return
	} else {
		log.Println("Get Image From Memcache: ", err)
	}
	log.Println("Get Image From Filesystem: ", pid, mode)
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
	imaging.Encode(buffer, dstImage, imaging.WEBP, 50)
	data, err := ioutil.ReadAll(buffer)
	if err != nil {
		log.Fatal("ReadAll: ", err.Error())
		return
	}
	memc.Set(&memcache.Item{Key: getKey(pid, mode), Value: data, Expiration: 86400})
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
	if r.Method != "POST" {
		return
	}
	log.Print("start upload image...")
	file, _, err := r.FormFile("image")
	if err != nil {
		log.Fatal("FormFile: ", err.Error())
		return
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Fatal("Close: ", err.Error())
		}
	}()

	img, err := imaging.Decode(file)
	if err != nil {
		log.Fatal("Decode: ", err.Error())
		return
	}

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
		log.Fatal("Save: ", err.Error())
		return
	}

	fmt.Fprint(w, "{\"pid\":\""+pid+"\"}")

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
	return imaging.Encode(file, img, imaging.WEBP, 75)
}

func getImageFromPid(pid string) (*os.File, error) {
	filename, _ := getFilename(pid)
	return os.Open(filename)

}
