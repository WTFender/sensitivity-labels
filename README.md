# sensitivity-labels
Get and set Microsoft sensitivity labels.

```
usage:
        labels.exe [--flags] get [path]
        labels.exe [--flags] set [path] [labelId] [tenantId]

commands
        get: list sensitivity labels for the provided file or directory
        set: apply the provided sensitivity label ID to the provided file or directory

arguments
        path: path to the file or directory
        labelId: sensitivity label ID to apply
        tenantId: microsoft tenant ID to apply

flags
        --labeled: only show files with labels
        --summary: show summary of results
        --recurse: recurse through subdirectory files
        --dry-run: show results of set command without applying
        --tmp-dir: temporary directory for file extraction
        --verbose: show diagnostic output

examples
        labels.exe --recursive --labeled get "c:\path\to\directory"
        labels.exe --summary set "c:\path\to\file.docx" "1234-1234-1234" "4321-4321-4321"
```

### about
1. Find supported file archives (xlsx, docx, pptx)
2. Extract each archive to a temporary directory
3. Read labels from tmpDir/docMetadata/LabelInfo.xml
4. (optional) Modify `id` (labelId) and `siteId` (tenantId)
5. Display results

## example LabelInfo.xml
```xml
<?xml version="1.0" encoding="utf-8" standalone="yes"?>
<clbl:labelList xmlns:clbl="http://schemas.microsoft.com/office/2020/mipLabelMetadata">
  <clbl:label id="{c55117b6-35e7-4866-8da0-8aeab17385d2}" enabled="1" method="Privileged" siteId="{37b1cb57-8023-4b88-bae9-2b532b0b70a6}" contentBits="0" removed="0" />
</clbl:labelList>
```