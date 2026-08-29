CC = gcc
SRCS = src/main.c
TARGET = bin/inline_app

all: $(TARGET)

$(TARGET): $(SRCS) ; $(CC) -o $@ $(SRCS)

clean: ; rm -f $(TARGET)
