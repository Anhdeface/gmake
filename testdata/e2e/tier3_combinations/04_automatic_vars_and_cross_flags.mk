CC = gcc
CFLAGS = -Wall -Wextra -O2
INCLUDES = -Iinclude -Isrc/internal
LIBS = -lm -lpthread -ldl

SRCS = src/module_a.c src/module_b.c
TARGET = bin/advanced_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^ $(LIBS)
