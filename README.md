# Gomake

**Phiên bản: 0.2.1 (Beta)**

[Tiếng Việt](README.md) | [English](README_en.md)

**Gomake** là một công cụ chuyển đổi hai chiều (bi-directional transpiler & converter) gọn nhẹ, độc lập hoàn toàn (zero-dependency) được viết bằng ngôn ngữ Go. Gomake giúp định nghĩa cấu trúc biên dịch dự án C/C++ bằng cú pháp cấu hình trực quan (`.gomake`) và tự động sinh ra tệp **GNU Makefile tiêu chuẩn**, hỗ trợ đa mục tiêu (multi-target) và theo dõi phụ thuộc tệp header tự động.

---

## Tại sao nên dùng Gomake?

Viết và duy trì Makefile thủ công thường tiềm ẩn nhiều vấn đề:
* Nhầm lẫn cú pháp giữa ký tự Tab và khoảng trắng (Space).
* Thiếu thiết lập theo dõi phụ thuộc header (`-MMD -MP`), dẫn đến việc sửa tệp `.h` nhưng mã nguồn không tự biên dịch lại nếu không chạy lệnh làm sạch.
* Xung đột tệp đối tượng (`.o`) khi dự án có nhiều mục tiêu cùng chứa các tệp trùng tên (như `main.c` hay `utils.c`).
* Các hệ thống như CMake hay Meson thường quá phức tạp đối với các dự án C/C++ vừa và nhỏ, firmware nhúng hoặc công cụ dòng lệnh (CLI).

Gomake giải quyết các vấn đề trên thông qua tệp cấu hình đơn giản nhưng sinh ra Makefile tối ưu và chuẩn xác.

---

## Tính năng nổi bật

* **Chuyển đổi hai chiều (Bi-directional)**:
  * Biên dịch tệp cấu hình `.gomake` thành `Makefile`.
  * Dịch ngược `Makefile` có sẵn thành `.gomake` nhờ bộ phân tích cú pháp tĩnh độc lập.
* **Tự động theo dõi Header (Auto-Header Tracking)**: Tự động chèn cờ `-MMD -MP` và chỉ thị `-include *.d`, theo dõi chính xác từng thay đổi trong tệp header (`.h`/`.hpp`).
* **Ngăn ngừa xung đột tệp đối tượng**: Cơ chế phân vùng không gian tên theo mục tiêu (`.target.o`) loại bỏ nguy cơ ghi đè tệp đối tượng giữa các mục tiêu khác nhau.
* **Hỗ trợ đa dạng đầu ra**: Hỗ trợ sinh tệp thực thi (Executable), thư viện tĩnh (Static Library `.a`) và thư viện động (Shared Library `.so`).
* **Xử lý song song**: Tích hợp Goroutines biên dịch đồng thời hàng loạt tệp cấu hình (`gomake all`).
* **Không phụ thuộc bên ngoài (Zero-Dependency)**: Sử dụng thuần túy thư viện chuẩn của Go (Go Standard Library), đóng gói thành một tệp nhị phân duy nhất.

---

## Kiến trúc dự án

Dự án bao gồm các module chính:
* `internal/parser`: Phân tích cú pháp tệp `.gomake` theo từng dòng, quản lý danh sách mục tiêu `[const]`, bảo lưu câu lệnh script và nhận diện điểm kết thúc `./gomake`.
* `internal/generator`: Tiếp nhận cấu trúc dữ liệu và sinh ra GNU Makefile chuẩn, tự động cấu hình trình biên dịch, thư mục include, liên kết thư viện và quy tắc dọn dẹp (`clean`).
* `internal/converter`: Bộ phân tích cú pháp tĩnh độc lập bao gồm Lexer, AST Engine và Variable Expander (hỗ trợ 25+ hàm Make như `wildcard`, `patsubst`, biến điều kiện `ifeq`/`ifdef`...). Đảm nhiệm việc dịch ngược Makefile về `.gomake`.
* `main.go`: Điều hướng dòng lệnh (CLI Router) và quản lý tiến trình xử lý song song thông qua `sync.WaitGroup`.

---

## Cài đặt

### Cách 1: Biên dịch từ mã nguồn
Yêu cầu môi trường đã cài đặt Go:
```sh
git clone https://github.com/Anhdeface/gmake.git
cd gmake
./build.sh
```

### Cách 2: Cài đặt trực tiếp qua Go CLI
```sh
go install github.com/Anhdeface/gmake@latest
```

---

## Hướng dẫn sử dụng

### 1. Khởi tạo cấu hình mẫu
Tạo tệp `build.gomake` mẫu với 2 mục tiêu chuẩn (`app` và `test`):
```sh
./gomake genconfig
```

### 2. Biên dịch một tệp .gomake sang Makefile
Sinh tệp Makefile tương ứng (hậu tố mặc định: `<tên_file>.makefile`):
```sh
./gomake build.gomake
# Kết quả: build.gomake.makefile
```
Thực thi trực tiếp với GNU Make:
```sh
make -f build.gomake.makefile
```

### 3. Biên dịch hàng loạt (Song song)
Tìm kiếm và biên dịch đồng thời tất cả các tệp `*.gomake` trong thư mục hiện tại:
```sh
./gomake all
```

### 4. Dịch ngược Makefile có sẵn sang Gomake (convert)
Phân tích tĩnh tệp `Makefile` hiện có và chuyển đổi thành cấu hình `.gomake`:
```sh
./gomake convert -i Makefile -o build.gomake -f
```

Bảng tham số của lệnh `convert`:
| Cờ lệnh (Flag) | Viết tắt | Mặc định | Mô tả |
|---|---|---|---|
| `--input` | `-i` | `Makefile` | Đường dẫn Makefile đầu vào |
| `--output` | `-o` | `build.gomake` | Đường dẫn tệp `.gomake` xuất ra |
| `--force` | `-f` | `false` | Ghi đè nếu tệp đầu ra đã tồn tại |
| `--stdout` | `-s` | `false` | In trực tiếp nội dung ra thiết bị xuất chuẩn (stdout) |
| `--verbose` | `-v` | `false` | Hiển thị thông tin chi tiết trong quá trình chuyển đổi |

*Lưu ý kỹ thuật:* Trình chuyển đổi xử lý chính xác các cấu trúc Make phổ biến (trình biên dịch, cờ flags, includes, libs, tệp nguồn, scripts). Đối với các Makefile sử dụng kỹ thuật động phức tạp (như hàm `$(eval)` đa tầng hoặc pattern rules lồng nhau), công cụ sẽ áp dụng cơ chế bỏ qua an toàn (graceful fallback) để đảm bảo tệp cấu hình đầu ra luôn rõ ràng và dễ bảo trì.

---

## Đặc tả cú pháp .gomake

Cấu hình Gomake sử dụng các khối đặt trong cặp ngoặc vuông `[...]` và kết thúc bắt buộc bằng dòng `./gomake`:

```ini
[const]
app, test

[config.setup.app]
compiler = gcc
flags = -Wall -O2
name = my_app
[end]

[config.dependency.app]
sources = src/*.c
includes = include/
object.dpdcy = yes
build.type = executable
libs = -lm -lpthread
[end]

[config.scripts]
run = ./my_app
[end]

./gomake
```

### Chi tiết các trường cấu hình:
* `[const]`: Khai báo danh sách tên mục tiêu trên một dòng (phân tách bởi dấu phẩy).
* `[config.setup.<TARGET>]`:
  * `compiler`: Trình biên dịch (ví dụ: `gcc`, `g++`, `clang`). Mặc định: `gcc`.
  * `flags`: Cờ biên dịch (ví dụ: `-Wall -O3 -std=c11`).
  * `name`: Tên tệp nhị phân hoặc thư viện đầu ra.
* `[config.dependency.<TARGET>]`:
  * `sources`: Danh sách tệp nguồn (hỗ trợ ký tự đại diện `*`, ví dụ: `src/*.c`).
  * `includes`: Thư mục header (tự động chuyển đổi thành cờ `-I<đường_dẫn>`).
  * `object.dpdcy`: Đặt `yes` để biên dịch riêng từng tệp đối tượng kèm tính năng theo dõi phụ thuộc header (`-MMD -MP`).
  * `build.type`: Loại mục tiêu cần xây dựng: `executable` (mặc định), `static` (thư viện `.a`), hoặc `shared` (thư viện `.so`).
  * `libs`: Cờ thư viện liên kết (ví dụ: `-lm -lpthread`).
* `[config.scripts]`: Khai báo các lệnh tùy biến (ví dụ: `run = ./my_app`), được sinh dưới dạng `.PHONY` target trong Makefile.
* `./gomake`: Ký hiệu kết thúc tệp cấu hình (bắt buộc).

---

## Phạm vi sử dụng phù hợp

* Dự án C/C++ quy mô vừa và nhỏ.
* Lập trình nhúng (Embedded Systems), vi điều khiển sử dụng bộ công cụ GCC.
* Các ứng dụng tiện ích dòng lệnh (CLI).
* Môi trường học tập và giảng dạy cần Makefile chuẩn mực mà không phải quản lý cú pháp phức tạp thủ công.

---

## Kiểm thử chất lượng

Dự án được kiểm thử toàn diện với hơn 2,800 dòng mã kiểm thử:
* **Unit Tests**: Kiểm thử chi tiết các thành phần Lexer, Parser, AST, Serializer và Converter.
* **Stress Tests**: Kiểm tra khả năng mở rộng biến đệ quy và ngăn ngừa tham chiếu vòng.
* **Hệ thống kiểm thử E2E 4 tầng (4-Tier Verification Suite)**:
  * Tầng 1: Kiểm thử độ bao phủ tính năng (Executable, Static Library, Shared Library, Header Tracking).
  * Tầng 2: Kiểm thử các trường hợp biên (khoảng trắng bất quy tắc, ký tự tiếp dòng, dấu chấm phẩy nội dòng, mục tiêu trùng lặp).
  * Tầng 3: Kiểm thử kết hợp nhiều mục tiêu hỗn hợp.
  * Tầng 4: Kiểm thử trên cấu trúc dự án thực tế (ứng dụng CLI, thư viện mã hóa, dịch vụ đa module).

Chạy toàn bộ kiểm thử:
```sh
go test -v ./...
```

---

## Bản quyền

Dự án được phân phối theo giấy phép MIT. Xem tệp [LICENSE](LICENSE) để biết thêm chi tiết.
