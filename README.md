# Gomake

**Version: 0.1.3 (Beta)**

Vietnamese | [English](README_en.md)

Gomake là một trình biên dịch chuyển đổi (transpiler) được viết bằng ngôn ngữ Go, có chức năng phân tích các tệp cấu hình tùy chỉnh (`.gomake`) và sinh ra tệp GNU Makefile tiêu chuẩn.

## Kiến trúc phần mềm

Dự án bao gồm ba thành phần module chính:
- **Parser (`internal/parser`)**: Phân tích cú pháp tệp `.gomake` theo từng dòng, bỏ qua các đoạn văn bản bắt đầu bằng `//`, trích xuất các khóa và giá trị từ khối `[config.setup]` và `[config.dependency]`. Quá trình phân tích sẽ kết thúc ngay khi gặp cờ báo EOF định nghĩa là `./gomake`.
- **Generator (`internal/generator`)**: Tiếp nhận cấu trúc dữ liệu đã phân tích và tạo tệp Makefile. Trình sinh mã thực hiện nối cờ `-I` cho các thư mục `includes` và tự động thiết lập quy tắc liên kết tệp đối tượng (object file linking) thông qua biến `$(OBJS)`.
- **CLI Router (`main.go`)**: Chịu trách nhiệm phân luồng lệnh thực thi, quản lý quy trình tạo tệp đa luồng (multi-threading) thông qua `sync.WaitGroup` và cung cấp lệnh khởi tạo cấu hình mẫu.

## Hướng dẫn cài đặt

Thực thi lệnh sau để biên dịch tệp nhị phân từ mã nguồn:

```sh
go build -o gomake main.go
```

## Hướng dẫn sử dụng

### 1. Sinh tệp cấu hình mẫu

Lệnh sau sẽ tạo ra tệp `build.gomake` chứa các trường tham số trống kèm chú thích giải thích:

```sh
./gomake genconfig
```

### 2. Dịch cấu hình đơn lẻ

Biên dịch một tệp `.gomake` cụ thể. Đầu ra sẽ được lưu vào tệp mới với hậu tố `.makefile` (ví dụ: `filename.gomake.makefile`):

```sh
./gomake <filename.gomake>
```

### 3. Dịch cấu hình hàng loạt

Kích hoạt Goroutines để tìm kiếm và dịch toàn bộ các tệp có đuôi `.gomake` trong thư mục hiện tại theo cơ chế song song:

```sh
./gomake all
```

## Đặc tả cấu hình

Cấu trúc định dạng tệp sử dụng các khối logic đặt trong cặp ngoặc vuông. Trình phân tích bảo toàn tính nguyên bản của các tham số (không tự động can thiệp thêm cờ tối ưu như `-O2` nếu không có sẵn).

- `[config.setup]`: Lưu trữ cấu hình trình biên dịch, cờ biên dịch và tên dự án.
- `[config.dependency]`: Lưu trữ cấu hình tệp đích, đường dẫn mã nguồn (hỗ trợ mẫu tìm kiếm đại diện wildcard), đường dẫn thư viện (includes) và thiết lập cơ chế liên kết đối tượng (`object.dpdcy`).
- `//`: Chuỗi biểu thị nội dung chú thích.
- `./gomake`: Ký tự đánh dấu kết thúc tệp cấu hình.

Mẫu định dạng tệp `.gomake`:

```
[config.setup]
compiler = gcc
flags = -Wall -Wextra -O2
name = my_app
[end]
[config.dependency]
target = bin/program
sources = src/main.c src/driver.c
includes = include/*
object.dpdcy = yes
[end]
./gomake
```
