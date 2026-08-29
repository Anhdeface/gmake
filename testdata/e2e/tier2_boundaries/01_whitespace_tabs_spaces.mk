CC = gcc
CFLAGS = -Wall   -O2   

SRCS = src/main.c   src/worker.c  
TARGET = bin/whitespace_app

all: $(TARGET)

$(TARGET): $(SRCS)
    $(CC) $(CFLAGS) -o $@ $(SRCS)
