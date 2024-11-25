<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {goto} from "$app/navigation";
    import {currentIndex, currentItem, items} from "$lib/stores";
    import {currentPage} from "$lib/stores";
    import {onMount} from "svelte";


    function selectItem(index: number) {
        $currentIndex = index;
        $currentItem = $items[$currentIndex]
    }

    function openItem(index: number) {
        goto("/" + index, {replaceState: true})
    }

    // todo manipulations with $rowHeight

    function handleScroll() {
        const scrollHeight = document.documentElement.scrollHeight;
        const scrollTop = window.scrollY;
        const clientHeight = window.innerHeight;

        if (scrollTop + clientHeight >= scrollHeight - 200) {
            currentPage.update(n => n + 1);
        }
    }

    onMount(() => {
        window.addEventListener('scroll', handleScroll);
        return () => window.removeEventListener('scroll', handleScroll);
    });

    // let paginatedItems = $derived(liveRune(
    //         () => layoutDb.items
    //             .orderBy("order")
    //             .offset($currentPage * $pageSize)
    //             .limit($pageSize)
    //             .toArray(),
    //         currentPage,
    //         pageSize
    //     )
    // );

</script>

<Gallery images={$items} selectItem={selectItem} openItem={openItem}/>
