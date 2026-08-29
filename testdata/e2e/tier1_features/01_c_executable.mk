CC = gcc
CFLAGS = -Wall -O2
SRCS = src/main.c src/util.c
TARGET = bin/my_c_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) -o $@ $(SRCS)
