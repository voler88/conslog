# Define Go commands.
GOCMD = go
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOCOVER = $(GOCMD) tool cover
GOGET = $(GOCMD) get
GOTIDY = $(GOCMD) mod tidy

COVERAGE = coverprofile.txt
BENCH_PROCS = 4
BENCH_COUNT = 5

all: update clean test 

update:
	$(GOGET) -u ./...
	$(GOTIDY)

clean:
	$(GOCLEAN)
	rm -rf $(COVERAGE)

test:
	GOMAXPROCS=$(BENCH_PROCS) $(GOTEST) -race -v -bench=. -benchmem -count $(BENCH_COUNT) -coverprofile $(COVERAGE) ./...
	$(GOCOVER) -func=$(COVERAGE)
