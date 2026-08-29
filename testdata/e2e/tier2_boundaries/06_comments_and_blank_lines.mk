# This is a leading file header comment
# describing the build configuration

CC = gcc # compiler to use
CFLAGS = -Wall # enable warnings

# Source files
SRCS = src/main.c # main file

# Target binary
TARGET = bin/comment_app

all: $(TARGET) # default goal

$(TARGET): $(SRCS)
	# Compile step
	$(CC) $(CFLAGS) -o $@ $(SRCS) # output binary

# Clean target
clean:
	rm -f $(TARGET)
