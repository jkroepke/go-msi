package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func init() { cases["MSIDB-11B"] = caseMSIDB11B }

func caseMSIDB11B() {
	c := newCase("MSIDB-11", "stream-name-limit")
	defer c.close()
	data, _ := filepath.Abs(c.path("data.bin"))
	os.WriteFile(data, []byte("payload"), 0o644)
	for _, n := range []int{54, 55, 56, 57} {
		path, _ := filepath.Abs(c.path(fmt.Sprintf("bin%d.msi", n)))
		os.Remove(path)
		db, _ := openDB(path, dbCreate)
		query(db, "CREATE TABLE `Binary` (`Name` CHAR(72) NOT NULL, `Data` OBJECT NOT NULL PRIMARY KEY `Name`)")
		key := strings.Repeat("k", n)
		ins := insertStream(db, "Binary", key, data)
		com := commit(db)
		closeHandle(db)
		read := "n/a"
		if com == nil {
			db, _ = openDB(path, dbReadOnly)
			b, err := readStreamField(db, "SELECT `Data` FROM `Binary`", 1)
			read = fmt.Sprintf("%q err=%v", b, err)
			closeHandle(db)
		}
		c.logf("RESULT key %d chars -> decoded stream name %d chars (%d encoded units): insert %v, commit %v, read %s", n, n+7, len(encodeStreamName("Binary."+key, false)), ins, com, read)
	}
}
