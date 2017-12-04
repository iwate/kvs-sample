package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
)

func main() {
	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Cannot get working directory, %v\n", err)
	}

	files, err := GetFileChecksums(wd, []string{"."})
	if err != nil {
		log.Fatalf("Error in GetFileChecksums, %v\n", err)
	}

	fmt.Printf("%v\n", files)

	db, err = leveldb.OpenFile(".save/level.db", nil)
	if err != nil {
		log.Fatalf("Cannot open db file, %v\n", err)
	}
	defer db.Close()

	hits, misses, errs := Extract(files)

	log.Println("Hits")
	for _, hit := range hits {
		log.Printf("%s %s\n", hit.Path, hit.Checksum)
	}

	log.Println("Misses")
	for _, miss := range misses {
		if miss.Reason() == PassedNewer {
			log.Printf("Need Update %s %s\n", miss.PassedValue.Path, miss.PassedValue.Checksum)
			key := []byte(miss.PassedValue.Path)
			value, err := json.Marshal(miss.PassedValue)
			if err != nil {
				log.Printf("Error encode json, %v\n", err)
			}
			if err = db.Put(key, value, nil); err != nil {
				log.Printf("Error write db, %v\n", err)
			}
		} else {
			log.Printf("Timesrip!!! %s %s\n", miss.StoredValue.Path, miss.StoredValue.Checksum)
		}
	}
	log.Println("Errors")
	for _, err = range errs {
		log.Printf("%v\n", err)
	}
}

var db *leveldb.DB

func Extract(files []FileChecksum) (hits []FileChecksum, miss []CacheMiss, errs []error) {
	hits = []FileChecksum{}
	miss = []CacheMiss{}
	errs = []error{}
	for _, file := range files {
		key := []byte(file.Path)

		exist, err := db.Has(key, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		var stored *FileChecksum
		if exist {
			value, err := db.Get(key, nil)

			if err != nil {
				errs = append(errs, err)
				continue
			}

			err = json.Unmarshal(value, stored)
			if err != nil {
				errs = append(errs, err)
				continue
			}
		}

		if stored == nil || stored.Checksum == file.Checksum {
			hits = append(hits, file)
		} else {
			miss = append(miss, CacheMiss{file, *stored})
		}
	}
	return hits, miss, errs
}

type CacheMissReason int

const (
	PassedNewer CacheMissReason = iota
	StoredNewer
)

type CacheMiss struct {
	PassedValue FileChecksum
	StoredValue FileChecksum
}

func (miss CacheMiss) Reason() CacheMissReason {
	if miss.PassedValue.ModTime < miss.StoredValue.ModTime {
		return StoredNewer
	}
	return PassedNewer
}

// GetFileChecksums get all checksums in tree
func GetFileChecksums(dir string, ignorePrefixies []string) ([]FileChecksum, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	checksums := []FileChecksum{}
	for _, file := range files {
		name := file.Name()
		path := filepath.Join(dir, name)

		if file.IsDir() {
			if anyPrefix(name, ignorePrefixies) == false {
				c, err := GetFileChecksums(path, ignorePrefixies)
				if err != nil {
					return nil, err
				}
				checksums = append(checksums, c...)
			}
		} else {
			reader, err := os.OpenFile(path, os.O_RDONLY, 000)
			if err != nil {
				return nil, err
			}
			b, err := ioutil.ReadAll(reader)
			if err != nil {
				return nil, err
			}
			hash := sha256.Sum256(b)
			checksums = append(checksums, FileChecksum{
				Path:     path,
				Checksum: hex.EncodeToString(hash[:]),
				ModTime:  file.ModTime().UnixNano(),
			})
		}
	}
	return checksums, err
}

func anyPrefix(str string, prefixies []string) bool {
	for _, prefix := range prefixies {
		if strings.HasPrefix(str, prefix) {
			return true
		}
	}
	return false
}

// FileChecksum is summary of file
type FileChecksum struct {
	Path     string
	Checksum string
	ModTime  int64
}
