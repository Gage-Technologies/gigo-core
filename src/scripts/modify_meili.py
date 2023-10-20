import os
import meilisearch

client = meilisearch.Client(os.environ.get('MEILI_HOST', 'http://localhost:7700'), os.environ.get('MEILI_API_KEY', 'master_key'))

index = client.index('posts')

index.update_documents([])

