export GLOBAL_VAR = 123
unexport LOCAL_VAR
vpath %.c src
override EXTRA_FLAGS = -DEXTRA

ifeq ($(DEBUG), 1)
CFLAGS = -g -O0
else
CFLAGS = -O2
endif

include optional.mk
-include deps.mk

CC = gcc
SRCS = src/main.c
TARGET = bin/graceful_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CC) $(CFLAGS) -o $@ $(SRCS)
