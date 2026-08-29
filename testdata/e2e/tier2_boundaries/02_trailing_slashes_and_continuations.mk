CC = gcc
CFLAGS = -Wall \
         -Wextra \
         -O2

SRCS = src/a.c \
       src/b.c \
       src/c.c

TARGET = bin/multiline_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) \
	      -o $@ \
	      $(SRCS)
