# Define Go commands.
GOCMD = go
GOCLEAN = $(GOCMD) clean
GOTEST = $(GOCMD) test
GOCOVER = $(GOCMD) tool cover
GOGET = $(GOCMD) get
GOTIDY = $(GOCMD) mod tidy

# Define test variables.
COVER_FILE = coverprofile.txt
BENCH_PROCS = 4
BENCH_COUNT = 5

all: update clean test bench

update:
	$(GOGET) -u ./...
	$(GOTIDY)

clean:
	$(GOCLEAN)

test:
	rm -rf $(COVER_FILE)
	$(GOTEST) -coverprofile $(COVER_FILE) ./...
	$(GOCOVER) -func=$(COVER_FILE)

bench:
	GOMAXPROCS=$(BENCH_PROCS) $(GOTEST) -race -v -bench=. -benchmem -count $(BENCH_COUNT) -run=^# ./...
