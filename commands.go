package photoshare

import (
	"flag"
	"fmt"
	"github.com/codegangsta/negroni"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
)

// Serve runs the HTTP server
func Serve() {

	cfg, err := newConfig()
	if err != nil {
		log.Fatal(err)
	}
	defer cfg.close()

	runtime.GOMAXPROCS((runtime.NumCPU() * 2) + 1)

	n := negroni.Classic()
	n.UseHandler(cfg.getRouter())
	n.Run(fmt.Sprintf(":%d", cfg.ServerPort))

}

func storeFile(cfg *config,
	filename,
	title,
	contentType string,
	tags []string,
	userID int64) error {
	log.Println(title)
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	name := generateRandomFilename(contentType)
	err = cfg.filestore.store(file, filename, contentType)
	if err != nil {
		return err
	}
	photo := &photo{
		Title:    title,
		Filename: name,
		Tags:     tags,
		OwnerID:  userID,
	}
	if err := cfg.datamapper.createPhoto(photo); err != nil {
		return err
	}

	return nil
}

func scanDir(cfg *config, userID int64, baseDir, dirname string) {
	fileList, err := ioutil.ReadDir(dirname)
	if err != nil {
		log.Println(err)
	}
	for _, info := range fileList {
		if info.IsDir() {
			scanDir(cfg, userID, baseDir, path.Join(dirname, info.Name()))
		} else {
			filename := path.Join(dirname, info.Name())
			tags := strings.Split(strings.TrimSpace(dirname[len(baseDir):]), "/")
			ext := strings.ToLower(path.Ext(info.Name()))
			if ext != ".jpg" && ext != ".png" {
				continue
			}
			title := info.Name()[:len(info.Name())-4]
			var contentType string
			if ext == ".jpg" {
				contentType = "image/jpeg"
			} else {
				contentType = "image/png"
			}
			if err := storeFile(cfg, filename, title, contentType, tags, userID); err != nil {
				log.Println(err)
			}
		}
	}
}

// Import from a given directory. Subdirs will be tags. Title will be filename.
func Import() {

	email := flag.String("user", "", "User email address")
	dirname := flag.String("dir", "", "Directory")

	flag.Parse()

	fmt.Println(*email)
	fmt.Println(*dirname)

	cfg, err := newConfig()
	if err != nil {
		log.Fatal(err)
	}
	defer cfg.close()

	user, err := cfg.datamapper.getUserByEmail(*email)
	if err != nil {
		log.Fatal(err)
	}

	scanDir(cfg, user.ID, *dirname, *dirname)

}
