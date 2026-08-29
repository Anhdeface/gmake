#include "crypto.hpp"

namespace crypto {
    std::string sha256_dummy(const std::string& input) {
        return "hash:" + input;
    }
}
