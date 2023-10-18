import requests
import json
import semver

def fetch_latest_compatible_version(publisher, extension_name, vs_code_version):
    url = f"https://open-vsx.org/api/{publisher}/{extension_name}"
    response = requests.get(url)

    if response.status_code == 200:
        extension_data = json.loads(response.text)
        all_versions = extension_data.get("allVersions", [])

        # Sort by version in descending order
        all_versions.sort(key=lambda x: semver.VersionInfo.parse(x['version']), reverse=True)

        for version in all_versions:
            if 'engines' in version and 'vscode' in version['engines']:
                required_vscode_version = version['engines']['vscode']
                if semver.match(vs_code_version, required_vscode_version):
                    return version['version']

        print("No compatible version found.")
        return None
    else:
        print(f"Failed to fetch data: {response.status_code}")
        return None

# Replace with the desired publisher, extension name, and VS Code version
publisher = "ms-python"
extension_name = "python"
vs_code_version = "1.60.0"  # Replace with your VS Code version

latest_compatible_version = fetch_latest_compatible_version(publisher, extension_name, vs_code_version)

if latest_compatible_version:
    print(f"Latest compatible version: {latest_compatible_version}")
