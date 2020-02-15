PROGRAM := csvwatch
SRCS := $(shell find . -name '*.go')

$(PROGRAM): $(SRCS)
	go build -o $(PROGRAM) main.go

.PHONY: test
test: $(PROGRAM)
	./$(PROGRAM) -p 3001 -s ./_example/style.css -t ./_example/testdata.csv

.PHONY: clean
clean:
	rm ./$(PROGRAM)
