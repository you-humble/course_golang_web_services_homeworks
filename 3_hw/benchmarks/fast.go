package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"strconv"
	"strings"

	"hw3/user"

	"github.com/mailru/easyjson"
)

const (
	android string = "Android"
	msie    string = "MSIE"
)

var (
	androidByte []byte = []byte(android)
	msieByte    []byte = []byte(msie)
)

func FastSearch(out io.Writer) {
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	file, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	seenBrowsers := make(map[string]struct{}, 1000)

	var isAndroid, isMSIE bool = false, false
	var i int = -1
	var user user.User
	var line []byte

	writer.WriteString("found users:")
	writer.WriteByte('\n')
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		i++
		line = scanner.Bytes()
		if !bytes.Contains(line, androidByte) && !bytes.Contains(line, msieByte) {
			continue
		}

		if err := easyjson.Unmarshal(line, &user); err != nil {
			panic("unmarshal error: " + err.Error())
		}

		isAndroid, isMSIE = false, false
		for _, browser := range user.Browsers {
			if strings.Contains(browser, android) {
				isAndroid = true
				seenBrowsers[browser] = struct{}{}

			} else if strings.Contains(browser, msie) {
				isMSIE = true
				seenBrowsers[browser] = struct{}{}
			}
		}

		if isAndroid && isMSIE {
			email := strings.Replace(user.Email, "@", " [at] ", 1)
			writer.WriteRune('[')
			writer.WriteString(strconv.Itoa(i))
			writer.WriteRune(']')
			writer.WriteByte(' ')
			writer.WriteString(user.Name)
			writer.WriteByte(' ')
			writer.WriteRune('<')
			writer.WriteString(email)
			writer.WriteRune('>')
			writer.WriteByte('\n')
		}
	}

	if err := scanner.Err(); err != nil {
		panic("scanner error: " + err.Error())
	}

	writer.WriteString("\nTotal unique browsers ")
	writer.WriteString(strconv.Itoa(len(seenBrowsers)))
	writer.WriteByte('\n')
}
