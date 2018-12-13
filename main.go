package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"sync"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

func errorFatal(err error) {
	if err != nil {
		debug.PrintStack()
		log.Fatalln(err)
	}
}

func errorLog(err error) bool {
	if err != nil {
		log.Println(err)
		return true
	}
	return false
}

func encodeData(data interface{}) []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Encode values.
	err := enc.Encode(data)
	errorFatal(err)

	return buf.Bytes()
}

func decodeKeys(rawKeys []byte) ([][]byte, error) {
	var keys [][]byte

	buf := bytes.NewBuffer(rawKeys)
	dec := gob.NewDecoder(buf)
	err := dec.Decode(&keys)

	return keys, err
}

func printKeys(keys [][]byte) {
	for _, v := range keys {
		fmt.Println(string(v))
	}
}

func saveKey(db *leveldb.DB, key []byte) {
	var keys [][]byte
	keysStorageKey := []byte("fft_keys")

	trans, err := db.OpenTransaction()
	if err != nil {
		errorFatal(err)
	}

	if has, err := trans.Has(keysStorageKey, nil); !errorLog(err) {
		if has {
			var rawKeys []byte
			if rawKeys, err = trans.Get(keysStorageKey, nil); err == nil {
				keys, err = decodeKeys(rawKeys)
				if err == nil {
					// printKeys(keys)
				} else {
					log.Println("Keys data array is broken!")
					errorFatal(err)
				}
			} else {
				errorFatal(err)
			}
		} else {
			keys = make([][]byte, 0)
		}
	}

	keys = append(keys, key)
	encoded := encodeData(keys)

	err = trans.Put(keysStorageKey, encoded, nil)
	if !errorLog(err) {
		trans.Commit()
	} else {
		trans.Discard()
	}
}

func saveFft(keyPumpChan chan []byte, db *leveldb.DB, fftSlices [][]int32, key []byte) {
	trans, err := db.OpenTransaction()
	if err == nil {
		err = trans.Put(key, encodeData(fftSlices), nil)
		if !errorLog(err) {
			trans.Commit()
			keyPumpChan <- key
		} else {
			trans.Discard()
		}
	} else {
		errorLog(err)
		trans.Discard()
	}
}

func buildWorker(keyPumpChan chan []byte,
	workerID int, fft int, dir string,
	db *leveldb.DB, queueFiles *Queue, wg *sync.WaitGroup) {

	for {
		f, ok := queueFiles.Next()
		if !ok {
			break
		}

		file := *f

		if !file.IsDir() {
			log.Println("Worker", workerID, ": Processing file \"", file.Name(), "\"")

			//save with a key: "<fft>:<filename>"
			key := []byte(fmt.Sprint(fft, ":", file.Name()))

			if has, err := db.Has(key, nil); err == nil {
				if !has {
					fftSlices := getFFT(dir+"/"+file.Name(), fft)
					saveFft(keyPumpChan, db, fftSlices, key)
				}
			} else {
				errorLog(err)
			}
		}
	}
	wg.Done()
	log.Println("Worker №", workerID, "is done.")
}

func buildFFTs(fft int, dir string, db *leveldb.DB, procs int) {
	files, err := ioutil.ReadDir(dir)
	errorFatal(err)

	log.Println("Start with workers count: ", procs)

	queueFiles := NewQueue(files)

	var wg sync.WaitGroup
	var waitQuit sync.WaitGroup
	waitQuit.Add(1)
	wg.Add(procs)

	keyPumpChan := make(chan []byte, 100)
	doneChan := make(chan struct{})

	//Serve keys saving
	go func() {
		q := false

		for !q {
			select {
			case key := <-keyPumpChan:
				saveKey(db, key)
			case <-doneChan:
				q = true
			}
		}

		waitQuit.Done()
	}()

	for i := 0; i < procs; i++ {
		go buildWorker(keyPumpChan, i, fft, dir, db, queueFiles, &wg)
	}

	wg.Wait()

	close(doneChan)
	waitQuit.Wait()

	log.Println("Done.")
}

func main() {
	//Parse cmd arguments
	var dir string
	if len(os.Args) >= 2 {
		dir = os.Args[1]
	} else {
		dir = "./sounds"
	}

	// The returned DB instance is safe for concurrent use. Which mean that all
	// DB's methods may be called concurrently from multiple goroutine.
	o := &opt.Options{
		Compression: opt.NoCompression,
	}
	db, err := leveldb.OpenFile("./database", o)
	errorFatal(err)
	defer db.Close()

	buildFFTs(1024, dir, db, runtime.NumCPU())
}
