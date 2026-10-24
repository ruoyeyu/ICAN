// Bounded, isolated CI checks: never write into an oversized allocation.
#include <cstdint>
#include <cstdlib>
#include <iostream>
#include <limits>
#include <new>
#include <resolv.h>
#include <string>

int main(int argc, char** argv) {
  if (argc != 2) return 2;
  if (std::string(argv[1]) == "allocation") {
    constexpr auto alignment = std::align_val_t{64};
    void* small = ::operator new(128, alignment);
    if (reinterpret_cast<std::uintptr_t>(small) % 64) return 1;
    ::operator delete(small, alignment);
    for (std::size_t delta : {0, 1, 31, 63, 64}) {
      volatile std::size_t size = std::numeric_limits<std::size_t>::max() - delta;
      for (bool array : {false, true}) {
        try {
          void* p = array ? ::operator new[](size, alignment) : ::operator new(size, alignment);
          if (array) ::operator delete[](p, alignment);
          else ::operator delete(p, alignment);
          std::cerr << "oversized allocation unexpectedly succeeded\n";
          return 1;
        } catch (const std::bad_alloc&) {}
      }
    }
    std::cout << "aligned allocation regression passed\n";
    return 0;
  }
  if (std::string(argv[1]) == "resolver") {
    // Exercise configuration parsing only. No DNS query or external traffic.
    const auto domain = std::string(63, 'a') + "." + std::string(63, 'b') +
                        "." + std::string(63, 'c') + "." + std::string(49, 'd');
    if (setenv("LOCALDOMAIN", domain.c_str(), 1) || res_init() != 0) return 1;
    res_close();
    std::cout << "long resolver search domain parsed without abort\n";
    return 0;
  }
  return 2;
}
