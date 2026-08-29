CC = gcc
CFLAGS = -Wall -Wextra -O2 -Iinclude
LIBS = -lpthread

SRCS_LIB = lib/server.c
SRCS_APP = src/main.c
SRCS_TEST = test/test_main.c

TARGET_LIB = lib/libserver.so
TARGET_APP = bin/server_daemon
TARGET_TEST = bin/test_runner

.PHONY: all clean run-test

all: $(TARGET_LIB) $(TARGET_APP) $(TARGET_TEST)

$(TARGET_LIB): $(SRCS_LIB)
	@mkdir -p lib
	$(CC) -shared -fPIC $(CFLAGS) -o $@ $(SRCS_LIB)

$(TARGET_APP): $(SRCS_APP)
	@mkdir -p bin
	$(CC) $(CFLAGS) -o $@ $(SRCS_APP) -Llib -lserver $(LIBS)

$(TARGET_TEST): $(SRCS_TEST)
	@mkdir -p bin
	$(CC) $(CFLAGS) -o $@ $(SRCS_TEST) -Llib -lserver $(LIBS)

run-test:
	./bin/test_runner

clean:
	rm -rf bin lib
