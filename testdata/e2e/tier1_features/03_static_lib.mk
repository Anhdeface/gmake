CC = gcc
CFLAGS = -Wall -O2
SRCS = src/math.c src/matrix.c
TARGET = lib/libmath.a

all: $(TARGET)

$(TARGET): $(SRCS)
	ar rcs $@ $(SRCS)
