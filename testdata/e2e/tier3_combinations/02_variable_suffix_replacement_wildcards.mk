CC = gcc
CFLAGS = -Wall -O2 -g
INCLUDES = -Iinclude

SRCS = $(wildcard src/*.c)
OBJS = $(SRCS:.c=.o)
TARGET = bin/wildcard_app

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) $(INCLUDES) -c $< -o $@
