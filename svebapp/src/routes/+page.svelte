<script lang="ts">
    import Gallery from "$lib/gallery/Gallery.svelte";
    import {goto} from "$app/navigation";
    import {currentIndex, currentItem, items} from "$lib/stores";

    //import layoutDb from "$lib/stores";


    function selectItem(index: number) {
        $currentIndex = index;
        $currentItem = $items[$currentIndex]
    }

    function openItem(index: number) {
        goto("/" + index, {replaceState: true})
    }


    // todo manipulations with $rowHeight


    /*
    async function loadPage() {
        const offset = currentPage * PAGE_SIZE;
        const newItems = await db.items
            .orderBy('createdAt')
            .offset(offset)
            .limit(PAGE_SIZE * 2) // Завантаження поточної та наступної сторінки
            .toArray();
        items = [...items, ...newItems];
        currentPage++;
    }

    function handleScroll() {
        if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 200) {
            loadPage();
        }
    }

    onMount(() => {
        loadPage();
        window.addEventListener('scroll', handleScroll);
        return () => window.removeEventListener('scroll', handleScroll);
    }); */

    //let items = liveQuery(() => layoutDb.items.orderBy('order').toArray());

</script>

<Gallery images={$items} selectItem={selectItem} openItem={openItem}/>
