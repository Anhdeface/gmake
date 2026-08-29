CXX = g++
CXXFLAGS = -std=c++17 -Wall -Wextra -O3 -Iinclude
SRCS = src/crypto.cpp
TARGET = lib/libcrypto_engine.a

.PHONY: all clean

all: $(TARGET)

$(TARGET): $(SRCS)
	@mkdir -p lib
	ar rcs $@ $(SRCS)

clean:
	rm -rf lib
