CC = gcc
CFLAGS = -Wall -Wextra -O2 -Iinclude
LIBS = -lm
SRCS = src/main.c src/logger.c
TARGET = bin/logger_cli

.PHONY: all clean run

all: $(TARGET)

$(TARGET): $(SRCS)
	@mkdir -p bin
	$(CC) $(CFLAGS) -o $@ $(SRCS) $(LIBS)

run:
	./bin/logger_cli

clean:
	rm -rf bin
