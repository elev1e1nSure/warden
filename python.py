a = float(input("Введите первое число: "))
op = input("Введите операцию (+, -, *, /): ")
b = float(input("Введите второе число: "))

if op == "+":
    print(f"Результат: {a + b}")
elif op == "-":
    print(f"Результат: {a - b}")
elif op == "*":
    print(f"Результат: {a * b}")
elif op == "/":
    if b == 0:
        print("Ошибка: деление на ноль")
    else:
        print(f"Результат: {a / b}")
else:
    print("Неизвестная операция")