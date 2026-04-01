import os
import sys
import json

def run():
    # Args are passed as a single JSON string in sys.argv[1]
    try:
        args = json.loads(sys.argv[1])
        path = args.get("path", ".")
        files = os.listdir(path)
        print(f"Files in {path}: {', '.join(files)}")
    except Exception as e:
        print(f"Error: {str(e)}")

if __name__ == "__main__":
    run()