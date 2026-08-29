CC = gcc
SRCS_BIN = src/bin_test.c
SRCS_PKG = src/pkg_test.c

all: bin/test pkg/test

bin/test: $(SRCS_BIN)
	$(CC) -o $@ $(SRCS_BIN)

pkg/test: $(SRCS_PKG)
	$(CC) -o $@ $(SRCS_PKG)
