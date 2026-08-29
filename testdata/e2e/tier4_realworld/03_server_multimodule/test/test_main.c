#include "server.h"
#include <assert.h>

int main(void) {
    assert(server_start(9000) == 0);
    return 0;
}
