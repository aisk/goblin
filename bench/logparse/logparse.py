# Log line parsing: string building, trimming, splitting, prefix tests and
# integer conversion, with the results counted in a dict.
lines = []
i = 0
while i < 4000:
    lines.append("  level=info user=u" + str(i % 97) + " ms=" + str(i % 31) + "  ")
    i = i + 1

users = {}
elapsed = 0
round_ = 0
while round_ < 280:
    j = 0
    while j < len(lines):
        fields = lines[j].strip().split(" ")
        k = 0
        while k < len(fields):
            field = fields[k]
            if field.startswith("user="):
                user = field[len("user="):]
                users[user] = users.get(user, 0) + 1
            if field.startswith("ms="):
                elapsed = elapsed + int(field[len("ms="):])
            k = k + 1
        j = j + 1
    round_ = round_ + 1
print(len(users), elapsed)
