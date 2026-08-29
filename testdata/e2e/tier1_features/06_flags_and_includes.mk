CC = gcc
CFLAGS = -Wall -Wextra -DDEBUG -Iinclude -I/usr/include/json-c
LIBS = -lm -lpthread -ljson-c
SRCS = src/main.c
TARGET = bin/service_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) $(SRCS) -o $@ $(LIBS)
