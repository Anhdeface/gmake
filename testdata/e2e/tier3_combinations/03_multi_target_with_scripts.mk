.PHONY: all clean run test install lint

CC = gcc
CXX = g++
CFLAGS = -Wall -O2
CXXFLAGS = -Wall -O2 -std=c++17

SRCS_APP = src/app.c
SRCS_TEST = test/test_suite.cpp

TARGET_APP = bin/web_app
TARGET_TEST = bin/test_runner

all: $(TARGET_APP) $(TARGET_TEST)

$(TARGET_APP): $(SRCS_APP)
	$(CC) $(CFLAGS) -o $@ $(SRCS_APP)

$(TARGET_TEST): $(SRCS_TEST)
	$(CXX) $(CXXFLAGS) -o $@ $(SRCS_TEST)

run:
	./bin/web_app --port 8080

test:
	./bin/test_runner --verbose

install:
	mkdir -p /usr/local/bin && cp bin/web_app /usr/local/bin/

lint:
	cppcheck --enable=all src/
