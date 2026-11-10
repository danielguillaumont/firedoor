# Pure, no-network shape validation for one Kubernetes policy-list response.
# Accepted representations are the typed API resource and kubectl's generic
# v1 List. This function NEVER converts an API error into confirmed absence.
function Assert-FiredoorV09PolicyListEnvelope {
    [CmdletBinding()]
    param([Parameter(Mandatory=$true)][object]$Document)

    if ($null -eq $Document -or $Document -isnot [pscustomobject]) {
        throw 'Kubernetes policy list is not a JSON object. Stop.'
    }
    foreach ($field in @('kind', 'apiVersion', 'items', 'metadata')) {
        if ($null -eq $Document.PSObject.Properties[$field]) {
            throw "Kubernetes policy list missing field $field. Stop."
        }
    }
    $kind = [string]$Document.kind
    $version = [string]$Document.apiVersion
    $typed = ($kind -ceq 'NetworkPolicyList' -and $version -ceq 'networking.k8s.io/v1')
    $generic = ($kind -ceq 'List' -and $version -ceq 'v1')
    if (-not ($typed -or $generic)) {
        throw "Unexpected Kubernetes list kind/apiVersion ($kind / $version). Stop."
    }
    if ($Document.items -isnot [System.Array]) {
        throw 'Kubernetes policy list items is not an array. Stop.'
    }
    if ($null -eq $Document.metadata -or $Document.metadata -isnot [pscustomobject]) {
        throw 'Kubernetes policy list metadata invalid. Stop.'
    }
    $continue = $Document.metadata.PSObject.Properties['continue']
    if ($null -ne $continue -and -not [string]::IsNullOrEmpty([string]$continue.Value)) {
        throw 'Kubernetes policy list is paginated; refusing incomplete results. Stop.'
    }
    foreach ($item in $Document.items) {
        if ($null -eq $item -or $item -isnot [pscustomobject]) {
            throw 'Kubernetes policy list includes an invalid item. Stop.'
        }
        if ($null -eq $item.PSObject.Properties['metadata'] -or
            $null -eq $item.metadata -or
            $item.metadata -isnot [pscustomobject]) {
            throw 'Kubernetes policy list contains an item without metadata. Stop.'
        }
        $itemKind = $item.PSObject.Properties['kind']
        $itemVersion = $item.PSObject.Properties['apiVersion']
        # Typed API lists need not repeat TypeMeta on nested objects.
        # Generic kubectl Lists MUST provide item kind and apiVersion.
        if ($generic -and ($null -eq $itemKind -or $null -eq $itemVersion)) {
            throw 'Generic Kubernetes list item lacks explicit resource type. Stop.'
        }
        if (($null -ne $itemKind -and $itemKind.Value -cne 'NetworkPolicy') -or
            ($null -ne $itemVersion -and $itemVersion.Value -cne 'networking.k8s.io/v1')) {
            throw 'Kubernetes policy list includes a foreign resource. Stop.'
        }
    }
}
