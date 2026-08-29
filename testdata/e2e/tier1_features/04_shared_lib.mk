CC = gcc
CFLAGS = -fPIC -Wall
SRCS = src/plugin.c src/hook.c
TARGET = lib/libplugin.so

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) -shared $(CFLAGS) -o $@ $(SRCS)
