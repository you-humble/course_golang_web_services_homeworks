package main

import (
	"log"
	"sort"
	"strconv"
	"strings"
)

// data --> job1(in1, out1) --> job2(in2=out1, out2) --> job3(in3=out2, out3) ... --> main(for range outN)
// --- PASS: (2.09s)
func ExecutePipeline(jobs ...job) {
	if len(jobs) == 0 {
		return
	}

	in := make(chan interface{})
	for _, j := range jobs {
		out := make(chan interface{})
		go func(j job, in, out chan interface{}) {
			defer close(out)
			j(in, out)
		}(j, in, out)

		in = out // in2=out1
	}

	for range in {
	}
}

func SingleHash(in, out chan interface{}) {
	sema := make(chan struct{}, 1)
	done := make(chan struct{})

	var count int
	for inData := range in {
		count++
		go func(inData interface{}) {
			out <- singleHash(sema, strconv.Itoa(inData.(int)))
			done <- struct{}{}
		}(inData)
	}

	for i := 0; i < count; i++ {
		<-done
	}
}

func singleHash(sema chan struct{}, data string) string {
	log.Printf("%s SingleHash data %s\n", data, data)
	md5Ch := make(chan string)
	go func() {
		sema <- struct{}{}
		md5Ch <- DataSignerMd5(data)
		<-sema
	}()

	crc32Ch := make(chan string)
	go func() {
		crc32Data := DataSignerCrc32(data)
		log.Printf("%s SingleHash crc32(data) %s\n", data, crc32Data)
		crc32Ch <- crc32Data
	}()

	crc32Md5Ch := make(chan string)
	go func() {
		md5Data := <-md5Ch
		log.Printf("%s SingleHash md5(data) %s\n", data, md5Data)
		crc32Md5Data := DataSignerCrc32(md5Data)
		log.Printf("%s SingleHash crc32(md5(data)) %s\n", data, crc32Md5Data)
		crc32Md5Ch <- crc32Md5Data
	}()

	crc32Data := <-crc32Ch
	crc32Md5Data := <-crc32Md5Ch
	result := crc32Data + "~" + crc32Md5Data
	log.Printf("%s SingleHash result %s\n", data, result)
	return result
}

func MultiHash(in, out chan interface{}) {
	done := make(chan struct{})

	var count int
	for inData := range in {
		count++
		go func(inData interface{}) {
			out <- multiHash(inData.(string))
			done <- struct{}{}
		}(inData)
	}

	for i := 0; i < count; i++ {
		<-done
	}
}

func multiHash(data string) string {
	const thCount int = 6
	resultSlice := make([]string, thCount)
	done := make(chan struct{})

	for i := 0; i < thCount; i++ {
		go func(i int) {
			th := strconv.Itoa(i)
			crc32ThData := DataSignerCrc32(th + data)
			resultSlice[i] = crc32ThData
			log.Printf("%s MultiHash crc32(th+step1): %s %s\n", data, th, crc32ThData)
			done <- struct{}{}
		}(i)
	}

	for i := 0; i < thCount; i++ {
		<-done
	}

	result := strings.Join(resultSlice, "")
	log.Printf("%s MultiHash result: %s\n", data, result)
	return result
}

func CombineResults(in, out chan interface{}) {
	dataList := make([]string, 0)
	for dataIface := range in {
		dataList = append(dataList, dataIface.(string))
	}

	sort.Strings(dataList)
	result := strings.Join(dataList, "_")
	log.Printf("CombineResults\nresult: %s\n", result)
	out <- result
}
