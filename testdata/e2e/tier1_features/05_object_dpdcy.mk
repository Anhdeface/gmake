CC = gcc
CFLAGS = -Wall -g
SRCS = src/app.c src/helper.c
OBJS = $(SRCS:.c=.o)
TARGET = bin/app_with_objs

all: $(TARGET)

$(TARGET): $(OBJS)
	$(CC) $(CFLAGS) -o $@ $^

%.o: %.c
	$(CC) $(CFLAGS) -c $< -o $@
