.PHONY: all clean run dummy

CC = gcc
SRCS = src/app.c
TARGET = bin/app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) -o $@ $(SRCS)

dummy:

clean:
	rm -f $(TARGET)

run:
	./$(TARGET)
