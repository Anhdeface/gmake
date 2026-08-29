package template

const ConfigTemplate = `// ==========================================
// Gomake Configuration Template
// Dấu '//' được sử dụng để viết comment
// ==========================================

[config.name]
// compiler: Trình biên dịch bạn muốn sử dụng (Ví dụ: gcc, clang, g++)
compiler = 

// flags: Các cờ biên dịch (Ví dụ: -Wall -Wextra -O2, -g)
flags = 

// name: Tên dự án của bạn
name = 
[end]

[config.dependency]
// target: Đường dẫn hoặc tên file thực thi sẽ được tạo ra (Ví dụ: bin/my_program)
target = 

// sources: Các file mã nguồn (Ví dụ: src/*.c, src/main.c src/utils.c)
sources = 

// includes: Thư mục chứa các file header (Ví dụ: include/*)
includes = 

// o.fix: Tự động quản lý và link các file object (.o). Điền 'yes' hoặc để trống (Mặc định hệ thống luôn tự động làm việc này).
o.fix = 
[end]

// Dòng dưới đây báo hiệu kết thúc file cấu hình và tiến hành build
./gomake
`
