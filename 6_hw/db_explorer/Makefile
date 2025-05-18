test:
	go test -v -race

docker-run:
	docker run -p 3306:3306 -v my-volume:/docker-entrypoint-initdb.d -e MYSQL_ROOT_PASSWORD=1234 -e MYSQL_DATABASE=golang -d mysql
