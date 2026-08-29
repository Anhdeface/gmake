CXX = g++
CXXFLAGS = -std=c++17 -Wall -O3
SRCS = src/main.cpp src/solver.cpp
TARGET = bin/solver_app

all: $(TARGET)

$(TARGET): $(SRCS)
	$(CXX) $(CXXFLAGS) -o $@ $(SRCS)
