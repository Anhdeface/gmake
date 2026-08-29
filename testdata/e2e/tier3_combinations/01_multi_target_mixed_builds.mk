CC = gcc
CFLAGS = -Wall -O2
INCLUDES = -Iinclude

SRCS_CORE = src/core.c
SRCS_PLUGIN = src/plugin.c
SRCS_CLI = src/cli.c

TARGET_LIB = lib/libcore.a
TARGET_SO = lib/libplugin.so
TARGET_APP = bin/main_cli

all: $(TARGET_LIB) $(TARGET_SO) $(TARGET_APP)

$(TARGET_LIB): $(SRCS_CORE)
	ar rcs $@ $(SRCS_CORE)

$(TARGET_SO): $(SRCS_PLUGIN)
	$(CC) -shared -fPIC $(CFLAGS) $(INCLUDES) -o $@ $(SRCS_PLUGIN)

$(TARGET_APP): $(SRCS_CLI)
	$(CC) $(CFLAGS) $(INCLUDES) -o $@ $(SRCS_CLI) -Llib -lcore
