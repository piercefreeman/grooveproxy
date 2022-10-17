# groove

## Development

If you'd like to continuously compile changes to the golang proxy in the background:

```
cd groove-python && poetry run watchmedo shell-command --command="cd ../ && bash ./build.sh" ../proxy
```
